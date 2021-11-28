module View exposing (View, html, map, placeholder)

import Html.Styled as Html exposing (Html)


type alias View msg =
    { title : String
    , body : List (Html msg)
    }


map : (msg1 -> msg2) -> View msg1 -> View msg2
map fn doc =
    { title = doc.title
    , body = List.map (Html.map fn) doc.body
    }


placeholder : String -> View msg
placeholder moduleName =
    { title = "Placeholder"
    , body = [ Html.text moduleName ]
    }


html : String -> List (Html msg) -> View msg
html title body =
    { title = "Aspargesgården | " ++ title
    , body = body
    }
