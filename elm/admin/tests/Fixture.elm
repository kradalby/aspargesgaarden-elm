module Fixture exposing (PhotoSpec, event, eventWith, photo, photoWith, spec)

{-| Server answers for the update tests, decoded as the app decodes them.
-}

import Api exposing (Event, Photo)
import Json.Decode as D


type alias PhotoSpec =
    { id : Int, width : Int, height : Int, alt : String, photographer : Maybe Int, state : String }


spec : Int -> PhotoSpec
spec id =
    { id = id, width = 1300, height = 900, alt = "Bilde " ++ String.fromInt id, photographer = Nothing, state = "ready" }


photo : Int -> Photo
photo id =
    photoWith (spec id)


photoWith : PhotoSpec -> Photo
photoWith p =
    case D.decodeString Api.photoDecoder (photoJson p) of
        Ok ph ->
            ph

        Err e ->
            Debug.todo (D.errorToString e)


photoJson : PhotoSpec -> String
photoJson p =
    """{ "id": """
        ++ String.fromInt p.id
        ++ """, "width": """
        ++ String.fromInt p.width
        ++ """, "height": """
        ++ String.fromInt p.height
        ++ """, "alt": """
        ++ quote p.alt
        ++ """, "caption": "", "photographer": """
        ++ (case p.photographer of
                Just ph ->
                    """{ "id": """ ++ String.fromInt ph ++ """, "name": "Ola", "url": "" }"""

                Nothing ->
                    "null"
           )
        ++ """, "previewUrl": "/img/p.jpg", "thumbUrl": "/img/t.jpg",
      "crops": { "card": { "x": 290, "y": 0, "w": 720, "h": 900 },
                 "banner": { "x": 0, "y": 84, "w": 1300, "h": 731 },
                 "share": { "x": 0, "y": 109, "w": 1300, "h": 682 } },
      "quality": {
        "card": { "minWidth": 480, "sharpWidth": 768, "soft": false },
        "banner": { "minWidth": 800, "sharpWidth": 896, "soft": false },
        "share": { "minWidth": 1200, "sharpWidth": 1200, "soft": false },
        "full": { "minWidth": 480, "sharpWidth": 992, "soft": false } },
      "status": { "state": """
        ++ quote p.state
        ++ """, "done": 0, "total": 0, "previewReady": true, "reason": "" },
      "updatedAt": "2026-11-09T17:02:10Z" }"""


quote : String -> String
quote s =
    "\"" ++ s ++ "\""


{-| A draft as the server sends it.
-}
event : { id : Int, updatedAt : String, summary : String } -> Event
event e =
    eventWith Nothing e


eventWith : Maybe PhotoSpec -> { id : Int, updatedAt : String, summary : String } -> Event
eventWith main e =
    let
        json =
            """{ "id": """
                ++ String.fromInt e.id
                ++ """, "status": "draft", "slug": "kurs", "slugFrozen": false, "slugFromTitle": true, "year": 2026,
      "path": "/arrangementer/2026/kurs", "url": "http://127.0.0.1/arrangementer/2026/kurs",
      "dateText": "", "isOver": false, "title": "Kurs", "summary": """
                ++ quote e.summary
                ++ """, "body": "", "start": "2026-11-14", "startTime": "", "end": "", "endTime": "",
      "extraDates": [], "priceText": "", "ticketUrl": "", "showInMenu": false, "menuLabel": "",
      "cancelled": false, "mainPhoto": """
                ++ (main |> Maybe.map photoJson |> Maybe.withDefault "null")
                ++ """, "extraPhotos": [], "poster": null, "publishedAt": null,
      "createdAt": "2026-11-01T10:00:00Z", "updatedAt": """
                ++ quote e.updatedAt
                ++ " }"
    in
    case D.decodeString Api.eventDecoder json of
        Ok ev ->
            ev

        Err err ->
            Debug.todo (D.errorToString err)
