module Api exposing
    ( Crops
    , Error(..)
    , Event
    , EventInput
    , EventLists
    , Me
    , Photo
    , PhotoRef
    , PhotoStatus
    , Photographer
    , Problem
    , Qualities
    , Quality
    , Rect
    , UsedBy
    , copyEvent
    , createEvent
    , createPhotographer
    , deleteEvent
    , deletePhoto
    , encodeEventInput
    , errorDecoder
    , eventDecoder
    , getEvent
    , getEvents
    , getGallery
    , getMe
    , getPhoto
    , getPhotographers
    , getStatuses
    , photoDecoder
    , prepare
    , preview
    , publish
    , saveCrops
    , setGallery
    , unpublish
    , updateEvent
    , updatePhoto
    , updatePhotographer
    , upload
    )

{-| The admin API as documented in docs/admin-api.md: types, JSON, requests.
-}

import Dict exposing (Dict)
import File exposing (File)
import Http
import Json.Decode as D exposing (Decoder)
import Json.Encode as E



-- TYPES


type alias Me =
    { name : String, siteUrl : String }


type alias Rect =
    { x : Int, y : Int, w : Int, h : Int }


{-| Crop rectangles by kind ("card", "banner", "share"), in upright source
pixels.
-}
type alias Crops =
    Dict String Rect


type alias Photographer =
    { id : Int, name : String, url : String }


type alias Photo =
    { id : Int
    , width : Int
    , height : Int
    , alt : String
    , photographer : Maybe Photographer
    , previewUrl : String
    , thumbUrl : String
    , crops : Crops
    , quality : Qualities
    , status : PhotoStatus
    }


{-| A kind's sharpness rule as the server states it: the narrowest crop it
takes, the narrowest that is sharp on every screen, and whether what the
pages draw for this photo now is narrower than that.
-}
type alias Quality =
    { minWidth : Int, sharpWidth : Int, soft : Bool }


type alias Qualities =
    { card : Quality, banner : Quality, share : Quality, full : Quality }


{-| How far the server is with the photo's sizes; state is "preparing",
"ready" or "failed", reason a code when failed.
-}
type alias PhotoStatus =
    { state : String, done : Int, total : Int, previewReady : Bool, reason : String }


type alias Event =
    { id : Int
    , status : String
    , slug : String
    , slugFrozen : Bool
    , year : Int
    , path : String
    , url : String
    , dateText : String
    , isOver : Bool
    , title : String
    , summary : String
    , body : String
    , start : String
    , startTime : String
    , end : String
    , endTime : String
    , extraDates : List String
    , priceText : String
    , ticketUrl : String
    , showInMenu : Bool
    , menuLabel : String
    , cancelled : Bool
    , mainPhoto : Maybe Photo
    , extraPhotos : List Photo
    , updatedAt : String
    }


type alias EventLists =
    { upcoming : List Event, drafts : List Event, past : List Event }


type alias PhotoRef =
    { id : Int, alt : String, photographerId : Maybe Int }


type alias EventInput =
    { updatedAt : String
    , slug : String
    , title : String
    , summary : String
    , body : String
    , start : String
    , startTime : String
    , end : String
    , endTime : String
    , extraDates : List String
    , priceText : String
    , ticketUrl : String
    , showInMenu : Bool
    , menuLabel : String
    , cancelled : Bool
    , mainPhoto : Maybe PhotoRef
    , extraPhotos : List PhotoRef
    }


{-| One broken rule from the server's error list.
-}
type alias Problem =
    { code : String, field : Maybe String, photoId : Maybe Int }


type alias UsedBy =
    { id : Int, title : String }


{-| Failures the screens tell apart. Everything the server says about rules
arrives as Problems; the special 409s carry what the user needs next.
-}
type Error
    = Network
    | Problems (List Problem)
    | Conflict Event
    | PhotoInUse (List UsedBy)
    | NameTaken Photographer
    | Unexpected



-- DECODERS


rectDecoder : Decoder Rect
rectDecoder =
    D.map4 Rect (D.field "x" D.int) (D.field "y" D.int) (D.field "w" D.int) (D.field "h" D.int)


photographerDecoder : Decoder Photographer
photographerDecoder =
    D.map3 Photographer (D.field "id" D.int) (D.field "name" D.string) (D.field "url" D.string)


photoDecoder : Decoder Photo
photoDecoder =
    D.succeed Photo
        |> field "id" D.int
        |> field "width" D.int
        |> field "height" D.int
        |> field "alt" D.string
        |> field "photographer" (D.nullable photographerDecoder)
        |> field "previewUrl" D.string
        |> field "thumbUrl" D.string
        |> field "crops" (D.dict rectDecoder)
        |> field "quality" qualitiesDecoder
        |> field "status" statusDecoder


qualitiesDecoder : Decoder Qualities
qualitiesDecoder =
    let
        quality =
            D.map3 Quality (D.field "minWidth" D.int) (D.field "sharpWidth" D.int) (D.field "soft" D.bool)
    in
    D.map4 Qualities (D.field "card" quality) (D.field "banner" quality) (D.field "share" quality) (D.field "full" quality)


statusDecoder : Decoder PhotoStatus
statusDecoder =
    D.map5 PhotoStatus
        (D.field "state" D.string)
        (D.field "done" D.int)
        (D.field "total" D.int)
        (D.field "previewReady" D.bool)
        (D.field "reason" D.string)


eventDecoder : Decoder Event
eventDecoder =
    D.succeed Event
        |> field "id" D.int
        |> field "status" D.string
        |> field "slug" D.string
        |> field "slugFrozen" D.bool
        |> field "year" D.int
        |> field "path" D.string
        |> field "url" D.string
        |> field "dateText" D.string
        |> field "isOver" D.bool
        |> field "title" D.string
        |> field "summary" D.string
        |> field "body" D.string
        |> field "start" D.string
        |> field "startTime" D.string
        |> field "end" D.string
        |> field "endTime" D.string
        |> field "extraDates" (D.list D.string)
        |> field "priceText" D.string
        |> field "ticketUrl" D.string
        |> field "showInMenu" D.bool
        |> field "menuLabel" D.string
        |> field "cancelled" D.bool
        |> field "mainPhoto" (D.nullable photoDecoder)
        |> field "extraPhotos" (D.list photoDecoder)
        |> field "updatedAt" D.string


field : String -> Decoder a -> Decoder (a -> b) -> Decoder b
field name dec =
    D.map2 (|>) (D.field name dec)


problemDecoder : Decoder Problem
problemDecoder =
    D.map3 Problem
        (D.field "code" D.string)
        (D.maybe (D.field "field" D.string))
        (D.maybe (D.field "photoId" D.int))


{-| Reads an error body; the status picks out the 409s that carry data.
-}
errorDecoder : Int -> Decoder Error
errorDecoder status =
    D.field "errors" (D.list problemDecoder)
        |> D.andThen
            (\problems ->
                let
                    codes =
                        List.map .code problems
                in
                if status == 409 && List.member "conflict" codes then
                    D.map Conflict (D.field "current" eventDecoder)

                else if status == 409 && List.member "photo_in_use" codes then
                    D.map PhotoInUse
                        (D.field "usedBy" (D.list (D.map2 UsedBy (D.field "id" D.int) (D.field "title" D.string))))

                else if status == 409 && List.member "photographer_name_taken" codes then
                    D.oneOf
                        [ D.map NameTaken (D.field "existing" photographerDecoder)
                        , D.succeed (Problems problems)
                        ]

                else if List.isEmpty problems then
                    D.fail "empty error list"

                else
                    D.succeed (Problems problems)
            )



-- ENCODERS


encodeRect : Rect -> E.Value
encodeRect r =
    E.object [ ( "x", E.int r.x ), ( "y", E.int r.y ), ( "w", E.int r.w ), ( "h", E.int r.h ) ]


encodePhotoRef : PhotoRef -> E.Value
encodePhotoRef p =
    E.object
        [ ( "id", E.int p.id )
        , ( "alt", E.string p.alt )
        , ( "photographerId", maybe E.int p.photographerId )
        ]


encodeEventInput : EventInput -> E.Value
encodeEventInput e =
    E.object
        [ ( "updatedAt", E.string e.updatedAt )
        , ( "slug", E.string e.slug )
        , ( "title", E.string e.title )
        , ( "summary", E.string e.summary )
        , ( "body", E.string e.body )
        , ( "start", E.string e.start )
        , ( "startTime", E.string e.startTime )
        , ( "end", E.string e.end )
        , ( "endTime", E.string e.endTime )
        , ( "extraDates", E.list E.string e.extraDates )
        , ( "priceText", E.string e.priceText )
        , ( "ticketUrl", E.string e.ticketUrl )
        , ( "showInMenu", E.bool e.showInMenu )
        , ( "menuLabel", E.string e.menuLabel )
        , ( "cancelled", E.bool e.cancelled )
        , ( "mainPhoto", maybe encodePhotoRef e.mainPhoto )
        , ( "extraPhotos", E.list encodePhotoRef e.extraPhotos )
        ]


maybe : (a -> E.Value) -> Maybe a -> E.Value
maybe enc =
    Maybe.map enc >> Maybe.withDefault E.null



-- REQUESTS


base : String
base =
    "/admin/api/"


expect : (Result Error a -> msg) -> Decoder a -> Http.Expect msg
expect toMsg dec =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.GoodStatus_ _ body ->
                    -- 204 has no body; decoders for it accept null.
                    D.decodeString dec (orNull body) |> Result.mapError (\_ -> Unexpected)

                Http.BadStatus_ meta body ->
                    Err (D.decodeString (errorDecoder meta.statusCode) body |> Result.withDefault Unexpected)

                Http.NetworkError_ ->
                    Err Network

                Http.Timeout_ ->
                    Err Network

                Http.BadUrl_ _ ->
                    Err Unexpected


orNull : String -> String
orNull body =
    if String.isEmpty (String.trim body) then
        "null"

    else
        body


request : String -> String -> Http.Body -> (Result Error a -> msg) -> Decoder a -> Cmd msg
request method path body toMsg dec =
    Http.request
        { method = method
        , headers = []
        , url = base ++ path
        , body = body
        , expect = expect toMsg dec
        , timeout = Just 60000
        , tracker = Nothing
        }


get : String -> (Result Error a -> msg) -> Decoder a -> Cmd msg
get path =
    request "GET" path Http.emptyBody


getMe : (Result Error Me -> msg) -> Cmd msg
getMe toMsg =
    get "me" toMsg (D.map2 Me (D.field "name" D.string) (D.field "siteUrl" D.string))


getEvents : (Result Error EventLists -> msg) -> Cmd msg
getEvents toMsg =
    get "events" toMsg <|
        D.map3 EventLists
            (D.field "upcoming" (D.list eventDecoder))
            (D.field "drafts" (D.list eventDecoder))
            (D.field "past" (D.list eventDecoder))


getEvent : Int -> (Result Error Event -> msg) -> Cmd msg
getEvent id toMsg =
    get ("events/" ++ String.fromInt id) toMsg eventDecoder


createEvent : EventInput -> (Result Error Event -> msg) -> Cmd msg
createEvent input toMsg =
    request "POST" "events" (Http.jsonBody (encodeEventInput input)) toMsg eventDecoder


updateEvent : Int -> EventInput -> (Result Error Event -> msg) -> Cmd msg
updateEvent id input toMsg =
    request "PUT" ("events/" ++ String.fromInt id) (Http.jsonBody (encodeEventInput input)) toMsg eventDecoder


eventAction : String -> Int -> String -> (Result Error Event -> msg) -> Cmd msg
eventAction action id updatedAt toMsg =
    request "POST"
        ("events/" ++ String.fromInt id ++ "/" ++ action)
        (Http.jsonBody (E.object [ ( "updatedAt", E.string updatedAt ) ]))
        toMsg
        eventDecoder


publish : Int -> String -> (Result Error Event -> msg) -> Cmd msg
publish =
    eventAction "publish"


unpublish : Int -> String -> (Result Error Event -> msg) -> Cmd msg
unpublish =
    eventAction "unpublish"


copyEvent : Int -> (Result Error Event -> msg) -> Cmd msg
copyEvent id toMsg =
    request "POST" ("events/" ++ String.fromInt id ++ "/copy") Http.emptyBody toMsg eventDecoder


deleteEvent : Int -> (Result Error () -> msg) -> Cmd msg
deleteEvent id toMsg =
    request "DELETE" ("events/" ++ String.fromInt id) Http.emptyBody toMsg (D.succeed ())


{-| The page as the server renders it from the unsaved input; view is "page"
or "list", id the saved event being edited (its frozen address, and its
place in the list).
-}
preview : String -> Maybe Int -> EventInput -> (Result Error String -> msg) -> Cmd msg
preview view id input toMsg =
    Http.request
        { method = "POST"
        , headers = []
        , url = base ++ "preview"
        , body =
            Http.jsonBody
                (E.object
                    (( "view", E.string view )
                        :: ( "event", encodeEventInput input )
                        :: (id |> Maybe.map (\i -> [ ( "id", E.int i ) ]) |> Maybe.withDefault [])
                    )
                )
        , expect =
            Http.expectStringResponse toMsg <|
                \response ->
                    case response of
                        Http.GoodStatus_ _ body ->
                            Ok body

                        Http.BadStatus_ meta body ->
                            Err (D.decodeString (errorDecoder meta.statusCode) body |> Result.withDefault Unexpected)

                        _ ->
                            Err Network
        , timeout = Just 30000
        , tracker = Nothing
        }


{-| Uploads one file; progress arrives through Http.track with tracker.
-}
upload : String -> File -> (Result Error Photo -> msg) -> Cmd msg
upload tracker file toMsg =
    Http.request
        { method = "POST"
        , headers = []
        , url = base ++ "photos"
        , body = Http.multipartBody [ Http.filePart "file" file ]
        , expect = expect toMsg photoDecoder
        , timeout = Nothing
        , tracker = Just tracker
        }


getPhoto : Int -> (Result Error Photo -> msg) -> Cmd msg
getPhoto id toMsg =
    get ("photos/" ++ String.fromInt id) toMsg photoDecoder


{-| Statuses of the given photos, keyed by id; unknown ids are left out.
-}
getStatuses : List Int -> (Result Error (Dict Int PhotoStatus) -> msg) -> Cmd msg
getStatuses ids toMsg =
    get ("photos/status?ids=" ++ String.join "," (List.map String.fromInt ids))
        toMsg
        (D.field "statuses" (D.keyValuePairs statusDecoder)
            |> D.map (List.filterMap (\( k, v ) -> String.toInt k |> Maybe.map (\i -> ( i, v ))) >> Dict.fromList)
        )


prepare : Int -> (Result Error PhotoStatus -> msg) -> Cmd msg
prepare id toMsg =
    request "POST" ("photos/" ++ String.fromInt id ++ "/prepare") Http.emptyBody toMsg statusDecoder


updatePhoto : Int -> { alt : String, photographerId : Maybe Int } -> (Result Error Photo -> msg) -> Cmd msg
updatePhoto id p toMsg =
    request "PUT"
        ("photos/" ++ String.fromInt id)
        (Http.jsonBody (E.object [ ( "alt", E.string p.alt ), ( "photographerId", maybe E.int p.photographerId ) ]))
        toMsg
        photoDecoder


saveCrops : Int -> Crops -> (Result Error Photo -> msg) -> Cmd msg
saveCrops id crops toMsg =
    request "PUT"
        ("photos/" ++ String.fromInt id ++ "/crops")
        (Http.jsonBody (E.dict identity encodeRect crops))
        toMsg
        photoDecoder


deletePhoto : Int -> (Result Error () -> msg) -> Cmd msg
deletePhoto id toMsg =
    request "DELETE" ("photos/" ++ String.fromInt id) Http.emptyBody toMsg (D.succeed ())


getPhotographers : (Result Error (List Photographer) -> msg) -> Cmd msg
getPhotographers toMsg =
    get "photographers" toMsg (D.field "photographers" (D.list photographerDecoder))


encodePhotographer : { name : String, url : String } -> Http.Body
encodePhotographer p =
    Http.jsonBody (E.object [ ( "name", E.string p.name ), ( "url", E.string p.url ) ])


createPhotographer : { name : String, url : String } -> (Result Error Photographer -> msg) -> Cmd msg
createPhotographer p toMsg =
    request "POST" "photographers" (encodePhotographer p) toMsg photographerDecoder


updatePhotographer : Int -> { name : String, url : String } -> (Result Error Photographer -> msg) -> Cmd msg
updatePhotographer id p toMsg =
    request "PUT" ("photographers/" ++ String.fromInt id) (encodePhotographer p) toMsg photographerDecoder


getGallery : (Result Error (List Photo) -> msg) -> Cmd msg
getGallery toMsg =
    get "gallery" toMsg (D.field "photos" (D.list photoDecoder))


setGallery : List Int -> (Result Error (List Photo) -> msg) -> Cmd msg
setGallery ids toMsg =
    request "PUT"
        "gallery"
        (Http.jsonBody (E.object [ ( "photoIds", E.list E.int ids ) ]))
        toMsg
        (D.field "photos" (D.list photoDecoder))
