package main

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/marni/goigc"
)

type trackInfo struct {
	ID          int
	TrackLength float64
	Pilot       string
	Glider      string
	GliderID    string
	HDate       string
}

var (
	trackInfos []trackInfo  // Holds track information of uploaded igc files
	startTime  = time.Now() // Used for getting the uptime of the service
)

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
	if id >= len(trackInfos) {
		return 0, errors.New("index out of bounds")
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
	case "track_length":
		return strconv.FormatFloat(t.TrackLength, 'f', 6, 64), true
	default:
		return "", false
	}
}

func main() {

	router := gin.Default()

	router.GET("/paragliding/", func(c *gin.Context) {
		c.Redirect(301, "/paragliding/api")
	})

	api := router.Group("/paragliding/api")
	{
		api.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"uptime":  getUptime(),
				"info":    "Service for Paragliding tracks.",
				"version": "v1",
			})
		})

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

			track, err := igc.ParseLocation(url)
			if err != nil {
				return
			}

			points := track.Points
			trackLength := 0.0
			for i := 1; i < len(points); i++ {
				trackLength += points[i-1].Distance(points[i])
			}

			id := len(trackInfos)
			trackInfo := trackInfo{
				ID:          id,
				TrackLength: trackLength,
				Pilot:       track.Pilot,
				Glider:      track.GliderType,
				GliderID:    track.GliderID,
				HDate:       track.Header.Date.String(),
			}

			trackInfos = append(trackInfos, trackInfo)
			c.JSON(http.StatusOK, gin.H{"id": id})
		})

		api.GET("/track", func(c *gin.Context) {
			ids := make([]int, len(trackInfos))
			for i := range ids {
				ids[i] = i
			}
			c.JSON(http.StatusOK, ids)
		})

		api.GET("/track/:id", func(c *gin.Context) {
			id, err := getAndValidateID(c)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			trackInfo := trackInfos[id]

			c.JSON(http.StatusOK, gin.H{
				"H_date":       trackInfo.HDate,
				"pilot":        trackInfo.Pilot,
				"glider":       trackInfo.Glider,
				"glider_id":    trackInfo.GliderID,
				"track_length": trackInfo.TrackLength,
			})
		})

		api.GET("/track/:id/:field", func(c *gin.Context) {
			id, err := getAndValidateID(c)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}

			trackInfo := trackInfos[id]
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
			c.JSON(http.StatusOK, gin.H{
				"t_latest":   "<latest added timestamp>,",
				"t_start":    "<the first timestamp of the added track>, this will be the oldest track recorded",
				"t_stop":     "<the last timestamp of the added track>, this might equal to t_latest if there are no more tracks left",
				"tracks":     "[<id1>, <id2>, ...]",
				"processing": "<time in ms of how long it took to process the request>",
			})
		})

		api.GET("/ticker/:param", func(c *gin.Context) {

			path := c.Param("param")

			switch path {
			case "latest":
				// GET /api/ticker/latest
				// What: returns the timestamp of the latest added track
				// Response type: text/plain
				// Response code: 200 if everything is OK, appropriate error code otherwise.
				// Response: <timestamp> for the latest added track
				timestamp := "temporary timestamp REPLACE ME"
				c.String(http.StatusOK, timestamp)
			default:
				// GET /api/ticker/<timestamp>
				// What: returns the JSON struct representing the ticker for the IGC tracks. The first returned track should have the timestamp HIGHER than the one provided in the query. The array of track IDs returned should be capped at 5, to emulate "paging" of the responses. The cap (5) should be a configuration parameter of the application (ie. easy to change by the administrator).
				// Response type: application/json
				// Response code: 200 if everything is OK, appropriate error code otherwise.
				// timestamp := path
				// if !isValidTimestamp(timestamp) {
				// c.Status(http.StatusNotFound)
				// }
				c.JSON(http.StatusOK, gin.H{
					"t_latest":   "<latest added timestamp of the entire collection>,",
					"t_start":    "<the first timestamp of the added track>, this must be higher than the parameter provided in the query",
					"t_stop":     "<the last timestamp of the added track>, this might equal to t_latest if there are no more tracks left",
					"tracks":     "[<id1>, <id2>, ...]",
					"processing": "<time in ms of how long it took to process the request>",
				})
			}
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		// for running locally
		port = "8080"
	}

	router.Run(":" + port)
}
