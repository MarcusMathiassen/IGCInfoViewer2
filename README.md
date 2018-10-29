# IGCInfoViewer2


# GET paraglider/api
What: meta information about the API
Response type: application/json
Response code: 200
Body template

    {
        "uptime": <uptime>,
        "info": "Service for Paragliding tracks.",
        "version": "v1"
    }


# POST paraglider/api/track
# GET paraglider/api/track
# GET paraglider/api/track/<id>
# GET paraglider/api/track/<id>/<field>
# GET /admin/api/tracks_count
# DELETE /admin/api/tracks
# GET paraglider/api/ticker/
# GET paraglider/api/ticker/latest
# GET paraglider/api/ticker/<timestamp>