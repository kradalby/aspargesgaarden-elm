module Route exposing (Route(..), fromUrl, href, toString)

import Html exposing (Attribute)
import Html.Attributes as A
import Url exposing (Url)
import Url.Parser as P exposing ((</>), Parser)


type Route
    = Overview
    | NewEvent
    | EditEvent Int
    | Gallery


parser : Parser (Route -> a) a
parser =
    P.s "admin"
        </> P.oneOf
                [ P.map Overview P.top
                , P.map NewEvent (P.s "arrangementer" </> P.s "ny")
                , P.map EditEvent (P.s "arrangementer" </> P.int)
                , P.map Gallery (P.s "galleri")
                ]


{-| Nothing for an address the app does not know; Main shows "not found".
-}
fromUrl : Url -> Maybe Route
fromUrl url =
    -- "/admin/" and "/admin" both mean the overview.
    P.parse parser { url | path = stripTrailingSlash url.path }


stripTrailingSlash : String -> String
stripTrailingSlash path =
    if String.length path > 1 && String.endsWith "/" path then
        String.dropRight 1 path

    else
        path


toString : Route -> String
toString route =
    case route of
        Overview ->
            "/admin/"

        NewEvent ->
            "/admin/arrangementer/ny"

        EditEvent id ->
            "/admin/arrangementer/" ++ String.fromInt id

        Gallery ->
            "/admin/galleri"


href : Route -> Attribute msg
href =
    toString >> A.href
