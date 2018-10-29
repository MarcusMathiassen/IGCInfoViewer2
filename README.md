# IGCInfoViewer

Hosting on Heroku with mongoDB on mLab.

Hosted at: https://igcinfoviewer2.herokuapp.com/

# Currently working:
- GET paraglider/api
    api information
- POST paraglider/api/track
    Adds a new track
    request body:
    ```
    {
        "url": "<url>"
    }
    ```
- GET paraglider/api/track
    returns an array of all track ids
- GET paraglider/api/track/<id>
    returns a track by id
- GET paraglider/api/track/<id>/<field>
    return a field in a track
- GET /admin/api/tracks_count
    returns the amount of tracks stored
- DELETE /admin/api/tracks
    removes all stored trackes
- GET paraglider/api/ticker
    ticker information
- GET paraglider/api/ticker/latest
    latest ticker information
- GET paraglider/api/ticker/<timestamp>
    ticker information of a given timestamp

# Incomplete:
    - Anything with webhooks