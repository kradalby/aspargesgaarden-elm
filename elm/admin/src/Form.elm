module Form exposing
    ( Form
    , afterSave
    , empty
    , errorsFor
    , fieldId
    , fromEvent
    , move
    , removeAt
    , setAt
    , summaryLimit
    , takePhotos
    , toInput
    , withVersions
    )

{-| The event form's state and its rules: what is sent, which control a
server error belongs to. Kept apart from the view so the rules are tested.
-}

import Api exposing (Event, EventInput, Photo, PhotoRef, Problem)
import Text


{-| Everything the user can change, and nothing else, so two forms are equal
exactly when there is nothing to save.
-}
type alias Form =
    { slug : String
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


empty : Form
empty =
    { slug = ""
    , title = ""
    , summary = ""
    , body = ""
    , start = ""
    , startTime = ""
    , end = ""
    , endTime = ""
    , extraDates = []
    , priceText = ""
    , ticketUrl = ""
    , showInMenu = False
    , menuLabel = ""
    , cancelled = False
    , mainPhoto = Nothing
    , extraPhotos = []
    }


{-| An address that follows the title is an empty field: she has not
chosen one, and sending it empty keeps it following.
-}
fromEvent : Event -> Form
fromEvent e =
    { slug =
        if e.slugFromTitle && not e.slugFrozen then
            ""

        else
            e.slug
    , title = e.title
    , summary = e.summary
    , body = e.body
    , start = e.start
    , startTime = e.startTime
    , end = e.end
    , endTime = e.endTime
    , extraDates = e.extraDates
    , priceText = e.priceText
    , ticketUrl = e.ticketUrl
    , showInMenu = e.showInMenu
    , menuLabel = e.menuLabel
    , cancelled = e.cancelled
    , mainPhoto = Maybe.map ref e.mainPhoto
    , extraPhotos = List.map ref e.extraPhotos
    }


ref : Photo -> PhotoRef
ref p =
    { id = p.id, alt = p.alt, photographerId = Maybe.map .id p.photographer, updatedAt = p.updatedAt }


{-| The form's photos at the versions given, their words kept: what she
wrote is hers, but it now answers to the photo as stored.
-}
withVersions : List Photo -> Form -> Form
withVersions photos =
    mapRefs
        (\r ->
            case List.filter (\p -> p.id == r.id) photos of
                p :: _ ->
                    { r | updatedAt = p.updatedAt }

                [] ->
                    r
        )


{-| The form's photos with the description and photographer of the given
photos, as stored.
-}
takePhotos : List Photo -> Form -> Form
takePhotos photos =
    mapRefs
        (\r ->
            case List.filter (\p -> p.id == r.id) photos of
                p :: _ ->
                    ref p

                [] ->
                    r
        )


mapRefs : (PhotoRef -> PhotoRef) -> Form -> Form
mapRefs f form =
    { form | mainPhoto = Maybe.map f form.mainPhoto, extraPhotos = List.map f form.extraPhotos }


{-| The request body. A date row left empty is an unused "add a date" click,
not an invalid date, so it is not sent.
-}
toInput : String -> Form -> EventInput
toInput updatedAt f =
    { updatedAt = updatedAt
    , slug = f.slug
    , title = f.title
    , summary = f.summary
    , body = f.body
    , start = f.start
    , startTime = f.startTime
    , end = f.end
    , endTime = f.endTime
    , extraDates = List.filter (not << String.isEmpty) f.extraDates
    , priceText = f.priceText
    , ticketUrl = f.ticketUrl
    , showInMenu = f.showInMenu
    , menuLabel =
        if f.showInMenu then
            f.menuLabel

        else
            ""
    , cancelled = f.cancelled
    , mainPhoto = f.mainPhoto
    , extraPhotos = f.extraPhotos
    }


{-| Advice, not a rule: the short text shows in search results, which cut
it about here.
-}
summaryLimit : Int
summaryLimit =
    160


{-| The id of the control a server error belongs to, if the form has one.
-}
fieldId : Problem -> Maybe String
fieldId p =
    case ( p.field, p.photoId, p.code ) of
        ( Just "mainPhoto", Just id, "alt_required" ) ->
            Just ("alt-" ++ String.fromInt id)

        ( Just "extraPhotos", Just id, "alt_required" ) ->
            Just ("alt-" ++ String.fromInt id)

        ( Just f, _, _ ) ->
            case f of
                "title" ->
                    Just "tittel"

                "slug" ->
                    Just "adresse"

                "start" ->
                    Just "dato"

                "startTime" ->
                    Just "klokken"

                "end" ->
                    Just "siste-dag"

                "endTime" ->
                    Just "slutter"

                "extraDates" ->
                    Just "flere-datoer"

                "summary" ->
                    Just "kort-tekst"

                "body" ->
                    Just "hele-teksten"

                "priceText" ->
                    Just "pris"

                "ticketUrl" ->
                    Just "billettlenke"

                "menuLabel" ->
                    Just "menynavn"

                "mainPhoto" ->
                    Just "hovedbilde"

                "extraPhotos" ->
                    Just "flere-bilder"

                _ ->
                    Nothing

        _ ->
            Nothing


{-| The messages for one control.
-}
errorsFor : String -> List Problem -> List String
errorsFor id =
    List.filter (\p -> fieldId p == Just id) >> List.map Text.problem


{-| The form after a save answered with stored. If nothing was typed while
the request was out, the form becomes what the server stored. Otherwise the
newer typing is kept, and only the address as stored (trimmed, or empty
while it follows the title) and the photos' new versions are taken over, so
no keystroke is lost.
-}
afterSave : { sent : Form, current : Form, stored : Form } -> Form
afterSave { sent, current, stored } =
    let
        -- The save gave its photos new versions; her newer words answer to them.
        versioned =
            mapRefs
                (\r ->
                    case List.filter (\s -> s.id == r.id) (List.filterMap identity [ stored.mainPhoto ] ++ stored.extraPhotos) of
                        s :: _ ->
                            { r | updatedAt = s.updatedAt }

                        [] ->
                            r
                )
                current
    in
    if current == sent then
        stored

    else if current.slug == sent.slug then
        { versioned | slug = stored.slug }

    else
        versioned



-- LIST EDITING


setAt : Int -> a -> List a -> List a
setAt i x =
    List.indexedMap
        (\j y ->
            if i == j then
                x

            else
                y
        )


removeAt : Int -> List a -> List a
removeAt i xs =
    List.take i xs ++ List.drop (i + 1) xs


{-| Moves the item at i by delta places; a move past either end is
ignored.
-}
move : Int -> Int -> List a -> List a
move i delta xs =
    let
        j =
            i + delta
    in
    case ( List.drop i xs |> List.head, List.drop j xs |> List.head ) of
        ( Just a, Just b ) ->
            if j < 0 then
                xs

            else
                xs |> setAt i b |> setAt j a

        _ ->
            xs
