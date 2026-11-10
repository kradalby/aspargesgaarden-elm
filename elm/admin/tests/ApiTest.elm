module ApiTest exposing (suite)

import Api
import ApiExamples
import Dict
import Expect
import Json.Decode as D
import Json.Encode as E
import Test exposing (Test, describe, test)


photoJson : String
photoJson =
    """
    { "id": 12, "width": 4032, "height": 3024, "alt": "Låven",
      "photographer": { "id": 3, "name": "Ola", "url": "" },
      "previewUrl": "/img/p.jpg", "thumbUrl": "/img/t.jpg",
      "crops": { "card": { "x": 1012, "y": 0, "w": 2419, "h": 3024 } },
      "quality": {
        "card": { "minWidth": 480, "sharpWidth": 768, "soft": false },
        "banner": { "minWidth": 800, "sharpWidth": 896, "soft": true },
        "share": { "minWidth": 1200, "sharpWidth": 1200, "soft": false },
        "full": { "minWidth": 480, "sharpWidth": 992, "soft": false } },
      "status": { "state": "preparing", "done": 12, "total": 46, "previewReady": true, "reason": "" },
      "updatedAt": "2026-11-09T17:02:10Z" }
    """


eventJson : String
eventJson =
    """
    { "id": 7, "status": "draft", "slug": "julemarked", "slugFrozen": false, "slugFromTitle": true, "year": 2026,
      "path": "/arrangementer/2026/julemarked", "url": "https://aspargesgaarden.no/arrangementer/2026/julemarked",
      "dateText": "14. november 2026 på Aspargesgården", "isOver": false,
      "title": "Julemarked", "summary": "", "body": "## Hei", "start": "2026-11-14", "startTime": "12:00",
      "end": "", "endTime": "", "extraDates": ["2026-11-21"], "priceText": "", "ticketUrl": "",
      "showInMenu": true, "menuLabel": "", "cancelled": false,
      "mainPhoto": """ ++ photoJson ++ """, "extraPhotos": [],
      "publishedAt": null, "createdAt": "2026-11-01T10:00:00Z", "updatedAt": "2026-11-10T09:12:44Z" }
    """


{-| The keys of an EventInput, and of its main photo's reference.
-}
inputKeys : String -> Result D.Error ( List String, List String )
inputKeys =
    let
        keys =
            D.keyValuePairs D.value |> D.map (List.map Tuple.first >> List.sort)
    in
    D.decodeString (D.map2 Tuple.pair keys (D.field "mainPhoto" keys))


suite : Test
suite =
    describe "Api"
        [ test "decodes a photo with its status and crops" <|
            \_ ->
                D.decodeString Api.photoDecoder photoJson
                    |> Result.map (\p -> ( p.status.done, p.status.previewReady, Dict.get "card" p.crops |> Maybe.map .w ))
                    |> Expect.equal (Ok ( 12, True, Just 2419 ))
        , test "a failed photo carries its reason; an unknown state is followed" <|
            \_ ->
                let
                    stateOf st reason =
                        photoJson
                            |> String.replace "\"state\": \"preparing\"" ("\"state\": \"" ++ st ++ "\"")
                            |> String.replace "\"reason\": \"\"" ("\"reason\": \"" ++ reason ++ "\"")
                            |> D.decodeString Api.photoDecoder
                            |> Result.map (.status >> .state)
                in
                [ stateOf "failed" "damaged", stateOf "ready" "", stateOf "later" "" ]
                    |> Expect.equal [ Ok (Api.Failed "damaged"), Ok Api.Ready, Ok Api.Preparing ]
        , test "decodes each kind's widths from the server" <|
            \_ ->
                D.decodeString Api.photoDecoder photoJson
                    |> Result.map (\p -> ( p.quality.banner.minWidth, p.quality.banner.sharpWidth, p.quality.banner.soft ))
                    |> Expect.equal (Ok ( 800, 896, True ))
        , test "decodes an event as documented" <|
            \_ ->
                D.decodeString Api.eventDecoder eventJson
                    |> Result.map (\e -> ( e.updatedAt, e.extraDates, Maybe.map .id e.mainPhoto ))
                    |> Expect.equal (Ok ( "2026-11-10T09:12:44Z", [ "2026-11-21" ], Just 12 ))
        , describe "errors"
            [ test "several rule errors at once, with field and photo" <|
                \_ ->
                    D.decodeString (Api.errorDecoder 422)
                        """{"errors":[{"code":"title_required","field":"title"},{"code":"alt_required","field":"mainPhoto","photoId":12}]}"""
                        |> Expect.equal
                            (Ok
                                (Api.Problems
                                    [ { code = "title_required", field = Just "title", photoId = Nothing }
                                    , { code = "alt_required", field = Just "mainPhoto", photoId = Just 12 }
                                    ]
                                )
                            )
            , test "a conflict carries the stored event and the photos changed elsewhere" <|
                \_ ->
                    case D.decodeString (Api.errorDecoder 409) ("""{"errors":[{"code":"conflict","field":"mainPhoto","photoId":12}],"current":""" ++ eventJson ++ ""","photos":[""" ++ photoJson ++ "]}") of
                        Ok (Api.Conflict e photos) ->
                            Expect.equal ( e.id, List.map .updatedAt photos ) ( 7, [ "2026-11-09T17:02:10Z" ] )

                        other ->
                            Expect.fail (Debug.toString other)
            , test "a photo in use lists its events" <|
                \_ ->
                    D.decodeString (Api.errorDecoder 409) """{"errors":[{"code":"photo_in_use"}],"usedBy":[{"id":7,"title":"Julemarked"}]}"""
                        |> Expect.equal (Ok (Api.PhotoInUse [ { id = 7, title = "Julemarked" } ]))
            , test "a taken name offers the existing photographer" <|
                \_ ->
                    D.decodeString (Api.errorDecoder 409) """{"errors":[{"code":"photographer_name_taken","field":"name"}],"existing":{"id":3,"name":"Ola","url":""}}"""
                        |> Expect.equal (Ok (Api.NameTaken { id = 3, name = "Ola", url = "" }))
            , test "an empty list is not an error the app understands" <|
                \_ ->
                    D.decodeString (Api.errorDecoder 500) """{"errors":[]}""" |> Expect.err
            ]
        , test "encodes the input with every documented key" <|
            \_ ->
                Api.encodeEventInput
                    { updatedAt = "t"
                    , slug = ""
                    , title = "T"
                    , summary = ""
                    , body = ""
                    , start = "2026-11-14"
                    , startTime = ""
                    , end = ""
                    , endTime = ""
                    , extraDates = []
                    , priceText = ""
                    , ticketUrl = ""
                    , showInMenu = False
                    , menuLabel = ""
                    , cancelled = False
                    , mainPhoto = Just { id = 12, alt = "Låven", photographerId = Nothing, updatedAt = "2026-11-09T17:02:10Z" }
                    , extraPhotos = []
                    }
                    |> E.encode 0
                    |> inputKeys
                    |> Expect.equal (inputKeys ApiExamples.eventInput)
        , test "decodes the documented examples" <|
            \_ ->
                [ D.decodeString Api.eventDecoder ApiExamples.event |> Result.map .id
                , D.decodeString Api.photoDecoder ApiExamples.photo |> Result.map .id
                ]
                    |> List.map (Result.mapError D.errorToString)
                    |> Expect.equal [ Ok 7, Ok 12 ]
        , test "a photo reference carries the photo's version" <|
            \_ ->
                Api.encodeEventInput
                    { updatedAt = "", slug = "", title = "", summary = "", body = "", start = "", startTime = "", end = "", endTime = "", extraDates = [], priceText = "", ticketUrl = "", showInMenu = False, menuLabel = "", cancelled = False, mainPhoto = Just { id = 12, alt = "", photographerId = Nothing, updatedAt = "2026-11-09T17:02:10Z" }, extraPhotos = [] }
                    |> E.encode 0
                    |> D.decodeString (D.at [ "mainPhoto", "updatedAt" ] D.string)
                    |> Expect.equal (Ok "2026-11-09T17:02:10Z")
        , test "a photo reference without a photographer sends null" <|
            \_ ->
                Api.encodeEventInput
                    { updatedAt = "", slug = "", title = "", summary = "", body = "", start = "", startTime = "", end = "", endTime = "", extraDates = [], priceText = "", ticketUrl = "", showInMenu = False, menuLabel = "", cancelled = False, mainPhoto = Just { id = 12, alt = "", photographerId = Nothing, updatedAt = "" }, extraPhotos = [] }
                    |> E.encode 0
                    |> D.decodeString (D.at [ "mainPhoto", "photographerId" ] (D.null "none"))
                    |> Expect.equal (Ok "none")
        ]
