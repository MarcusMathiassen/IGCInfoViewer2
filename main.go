package main

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"

	"github.com/gin-gonic/gin"
	"github.com/marni/goigc"
)

var (
	startTime = time.Now() // Used for getting the uptime of the service
)

type trackInfo struct {
	ID          int     `bson:"id" json:"id"`
	TrackLength float64 `bson:"calculated total track length" json:"calculated total track length"`
	Pilot       string  `bson:"pilot" json:"pilot"`
	Glider      string  `bson:"glider" json:"glider"`
	GliderID    string  `bson:"glider_id" json:"glider_id"`
	HDate       string  `bson:"h_date" json:"h_date"`
	URL         string  `bson:"track_src_url" json:"url"`
	TimeStamp   string  `bson:"timestamp" json:"timestamp"`
}

func fmtDurationAsISO8601(duration time.Duration) string {
	days := int64(duration.Hours() / 24)
	years := days / 365
	months := years / 12
	hours := int64(math.Mod(duration.Hours(), 24))
	minutes := int64(math.Mod(duration.Minutes(), 60))
	seconds := int64(math.Mod(duration.Seconds(), 60))

	return fmt.Sprintf("P%dY%dM%dDT%dH%dM%dS", years, months, days, hours, minutes, seconds)
}

func getUptime() string {
	return fmtDurationAsISO8601(time.Since(startTime))
}

func getAndValidateID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (t trackInfo) getFieldByName(fieldName string) (string, bool) {
	switch fieldName {
	case "pilot":
		return t.Pilot, true
	case "glider":
		return t.Glider, true
	case "glider_id":
		return t.GliderID, true
	case "H_date":
		return t.HDate, true
	case "calculated total track length":
		return strconv.FormatFloat(t.TrackLength, 'f', 6, 64), true
	case "track_src_url":
		return t.URL, true
	case "timestamp":
		return t.TimeStamp, true
	default:
		return "", false
	}
}

type TrackDB struct {
	DatabaseURL    string `bson:"database_url"`
	DatabaseName   string `bson:"database_name"`
	CollectionName string `bson:"collection_name"`
}

func getCollection(db TrackDB) *mgo.Collection {
	session, err := mgo.Dial(db.DatabaseURL)
	if err != nil {
		panic(err)
	}
	return session.DB(db.DatabaseName).C(db.CollectionName)
}

/// Count ... returns the amount of tracks stored
func (db TrackDB) Count() int {
	count, err := getCollection(db).Count()
	if err != nil {
		panic(err)
	}
	return count
}

func (db TrackDB) GetLatestTrack() trackInfo {
	var latestTrack trackInfo
	err := getCollection(db).Find(bson.M{"id": db.Count() - 1}).One(&latestTrack)
	if err != nil {
		panic(err)
	}
	return latestTrack
}

// DeleteAllTracks ...deletes all tracks in the database
func (db TrackDB) GetTrackByID(id int) trackInfo {
	var track trackInfo
	err := getCollection(db).Find(bson.M{"id": id}).One(&track)
	if err != nil {
		panic(err)
	}
	return track
}

func (db TrackDB) Init() {
	session, err := mgo.Dial(db.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer session.Close()
}
func (db TrackDB) DeleteAllTracks() int {
	numDeleted := db.Count()
	if numDeleted == 0 {
		return 0
	}
	_, err := getCollection(db).RemoveAll(bson.M{})
	if err != nil {
		panic(err)
	}
	return numDeleted
}

func main() {

	db := TrackDB{
		DatabaseURL:    "mongodb://tester:test1234@ds145053.mlab.com:45053/igcinfoviewer",
		DatabaseName:   "igcinfoviewer",
		CollectionName: "Tracks",
	}

	db.Init()

	router := gin.Default()

	// /paragliding redirects to /paragliding/api
	router.GET("/paragliding/", func(c *gin.Context) {
		c.Redirect(301, "/paragliding/api")
	})

	adminAPI := router.Group("/admin/api")
	{
		// 		GET /admin/api/tracks_count
		// What: returns the current count of all tracks in the DB
		// Response type: text/plain
		// Response code: 200 if everything is OK, appropriate error code otherwise.
		// Response: current count of the DB records
		adminAPI.GET("/track_count", func(c *gin.Context) {
			numTracks := db.Count()
			c.String(http.StatusOK, strconv.Itoa(numTracks))
		})

		// 		DELETE /admin/api/tracks
		// What: deletes all tracks in the DB
		// Response type: text/plain
		// Response code: 200 if everything is OK, appropriate error code otherwise.
		// Response: count of the DB records removed from DB
		adminAPI.DELETE("/tracks", func(c *gin.Context) {
			numDeleted := db.DeleteAllTracks()
			c.String(http.StatusOK, strconv.Itoa(numDeleted))
		})
	}

	api := router.Group("/paragliding/api")
	{
		// 		GET /api
		// What: meta information about the API
		// Response type: application/json
		// Response code: 200
		api.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"uptime":  getUptime(),
				"info":    "Service for Paragliding tracks.",
				"version": "v1",
			})
		})

		// 		POST /api/track
		// What: track registration
		// Response type: application/json
		// Response code: 200 if everything is OK, appropriate error code otherwise,
		//  eg. when provided body content, is malformed or URL does not point to a proper IGC file,
		//  etc. Handle all errors gracefully.
		api.POST("/track", func(c *gin.Context) {
			var json map[string]interface{}
			var url string
			if c.BindJSON(&json) == nil {
				url = json["url"].(string)
			}
			if url == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing key 'url'"})
				return
			}

			if filepath.Ext(url) != ".igc" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "not a .igc file"})
				return
			}

			collection := getCollection(db)

			// Check if tracks already exists in database
			var existingTrack trackInfo
			trackExists := collection.Find(bson.M{"track_src_url": url}).One(&existingTrack)
			if trackExists == nil { // already exists
				c.JSON(http.StatusOK, gin.H{"id": existingTrack.ID})
				return
			}

			// Parse the track
			track, err := igc.ParseLocation(url)
			if err != nil {
				return
			}

			// Calculate track length
			points := track.Points
			trackLength := 0.0
			for i := 1; i < len(points); i++ {
				trackLength += points[i-1].Distance(points[i])
			}

			// Add to database
			id := db.Count()
			err = collection.Insert(trackInfo{
				ID:          id,
				TrackLength: trackLength,
				Pilot:       track.Pilot,
				Glider:      track.GliderType,
				GliderID:    track.GliderID,
				HDate:       track.Header.Date.String(),
				TimeStamp:   time.Now().String(),
				URL:         url,
			})
			if err != nil {
				panic(err)
			}
			c.JSON(http.StatusOK, gin.H{"id": id})
		})

		// GET /api/track
		// What: returns the array of all tracks ids
		// Response type: application/json
		// Response code: 200 if everything is OK, appropriate error code otherwise.
		// Response: the array of IDs, or an empty array if no tracks have been stored yet.
		api.GET("/track", func(c *gin.Context) {
			ids := make([]int, db.Count())
			for i := range ids {
				ids[i] = i
			}
			c.JSON(http.StatusOK, ids)
		})

		// GET /api/track/<id>
		// What: returns the meta information about a given track with the provided <id>, or NOT FOUND response code with an empty body.
		// Response type: application/json
		// Response code: 200 if everything is OK, appropriate error code otherwise.
		api.GET("/track/:id", func(c *gin.Context) {
			id, err := getAndValidateID(c)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}

			trackInfo := db.GetTrackByID(id)

			c.JSON(http.StatusOK, gin.H{
				"H_date":                        trackInfo.HDate,
				"pilot":                         trackInfo.Pilot,
				"glider":                        trackInfo.Glider,
				"glider_id":                     trackInfo.GliderID,
				"calculated total track length": trackInfo.TrackLength,
				"track_src_url":                 trackInfo.URL,
			})
		})

		// GET /api/track/<id>/<field>
		// What: returns the single detailed meta information about a given track with the provided <id>,
		//  or NOT FOUND response code with an empty body. The response should always be a string, with the exception of
		//  the calculated track length, that should be a number.
		// Response type: text/plain
		// Response code: 200 if everything is OK, appropriate error code otherwise.
		api.GET("/track/:id/:field", func(c *gin.Context) {
			id, err := getAndValidateID(c)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}

			trackInfo := db.GetTrackByID(id)
			fieldRequested, fieldExists := trackInfo.getFieldByName(c.Param("field"))
			if !fieldExists {
				c.Status(http.StatusNotFound)
				return
			}

			c.String(http.StatusOK, fieldRequested)
		})

		// GET /api/ticker/
		// What: returns the JSON struct representing the ticker for the IGC tracks. The first track returned should be the oldest. The array of track ids returned should be capped at 5, to emulate "paging" of the responses. The cap (5) should be a configuration parameter of the application (ie. easy to change by the administrator).
		// Response type: application/json
		// Response code: 200 if everything is OK, appropriate error code otherwise.
		api.GET("/ticker", func(c *gin.Context) {
			collection := getCollection(db)
			processingTimeStart := time.Now()
			var tracks []trackInfo
			err := collection.Find(bson.M{}).All(&tracks)
			if err != nil {
				panic(err)
			}
			numTracksToShow := 5
			if len(tracks) < numTracksToShow {
				numTracksToShow = len(tracks)
			}
			tracksToShow := tracks[len(tracks)-numTracksToShow : len(tracks)]
			ids := make([]int, numTracksToShow)
			for i := range tracksToShow {
				ids[i] = tracksToShow[i].ID
			}

			processingTimeSpent := time.Since(processingTimeStart).Seconds() * 1000
			c.JSON(http.StatusOK, gin.H{
				"t_latest":   db.GetLatestTrack().TimeStamp,
				"t_start":    tracksToShow[0].TimeStamp,
				"t_stop":     tracksToShow[numTracksToShow-1].TimeStamp,
				"tracks":     ids,
				"processing": processingTimeSpent,
			})
		})

		api.GET("/ticker/:param", func(c *gin.Context) {
			collection := getCollection(db)
			param := c.Param("param")
			switch param {
			case "latest":
				// GET /api/ticker/latest
				// What: returns the timestamp of the latest added track
				// Response type: text/plain
				// Response code: 200 if everything is OK, appropriate error code otherwise.
				// Response: <timestamp> for the latest added track
				c.String(http.StatusOK, db.GetLatestTrack().TimeStamp)
			default:
				// GET /api/ticker/<timestamp>
				// What: returns the JSON struct representing the ticker for the IGC tracks. The first returned track should have the timestamp HIGHER than the one provided in the query. The array of track IDs returned should be capped at 5, to emulate "paging" of the responses. The cap (5) should be a configuration parameter of the application (ie. easy to change by the administrator).
				// Response type: application/json
				// Response code: 200 if everything is OK, appropriate error code otherwise.
				processingTimeStart := time.Now()
				var tracks []trackInfo
				err := collection.Find(bson.M{}).All(&tracks)
				if err != nil {
					panic(err)
				}
				inputTimestamp := param
				var track trackInfo
				err = collection.Find(bson.M{"timestamp": inputTimestamp}).One(&track)
				if err != nil {
					c.Status(http.StatusNotFound)
					return
				}
				idOfQuery := tracks[track.ID].ID

				trackCount := len(tracks)

				numTracksToShow := 5

				rangeMin := idOfQuery
				rangeMax := idOfQuery + numTracksToShow
				if rangeMax > trackCount {
					rangeMax = trackCount
				}
				tracksToShow := tracks[rangeMin:rangeMax]
				ids := make([]int, rangeMax-rangeMin)
				for i := range tracksToShow {
					ids[i] = tracksToShow[i].ID
				}

				processingTimeSpent := time.Since(processingTimeStart).Seconds() * 1000
				c.JSON(http.StatusOK, gin.H{
					"t_latest":   db.GetLatestTrack().TimeStamp,
					"t_start":    tracksToShow[0].TimeStamp,
					"t_stop":     tracksToShow[len(tracksToShow)-1].TimeStamp,
					"tracks":     ids,
					"processing": processingTimeSpent,
				})
			}
		})
	}

	port := os.Getenv("PORT")
	if port == "" { // for running locally
		port = "8080"
	}

	router.Run(":" + port)
}
