module View.Footer exposing (..)

import Css
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, css, href, rel, src, target)
import Tailwind.Utilities as Tw


view : Html msg
view =
    footer
        [ css
            [ Tw.w_full
            , Tw.text_center
            , Tw.p_4
            ]
        ]
        [ div []
            [ contactPoint "https://facebook.com/aspargesgaarden" "/ikoner/fb.png" "Facebook"
            , contactPoint "https://instagram.com/aspargesgaarden" "/ikoner/insta.png" "Instagram"
            ]
        , node "script"
            [ src "https://umami.kradalby.no/umami.js"
            , attribute "data-website-id" "168bd9f6-fec6-40dc-95b5-b0e4082c7a98"
            , attribute "async" ""
            , attribute "defer" ""
            ]
            []
        ]


contactPoint : String -> String -> String -> Html msg
contactPoint url imagePath altName =
    a
        [ css
            [ Tw.inline_block
            , Tw.w_14
            , Tw.h_14
            , Tw.p_2
            , Tw.transform
            , Tw.transition
            , Tw.duration_200
            , Tw.ease_in_out
            , Css.hover [ Tw.scale_125 ]
            ]
        , href url
        , target "_blank"
        , rel "noopener noreferrer"
        ]
        [ img [ src imagePath, alt altName ] [] ]
