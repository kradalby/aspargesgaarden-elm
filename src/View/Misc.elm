module View.Misc exposing (contact, container, headline, paragraph, viewIf)

import Css
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, css, href, src)
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View


contact : Html msg
contact =
    div
        [ css
            [ Bp.lg [ Tw.w_44 ]
            , Tw.w_36
            , Tw.py_3
            , Tw.mx_auto
            ]
        ]
        [ a [ href "/contact" ]
            [ img [ src "/ikoner/takontakt.png", alt "Ta kontakt" ] []
            ]
        ]


paragraph : String -> Html msg
paragraph content =
    p [ css [ Tw.pb_3 ] ] [ text content ]


headline : String -> Html msg
headline content =
    h1
        [ css
            [ Bp.xl [ Tw.pt_12 ]
            , Bp.lg [ Tw.pt_12 ]
            , Bp.md
                [ Tw.text_left
                , Tw.text_8xl
                ]
            , Tw.text_center
            , Tw.pt_10
            , Tw.text_6xl
            , Tw.py_6
            , Css.fontFamilies [ "luxus-brut", "cursive" ]
            , Tw.text_brown
            ]
        ]
        [ text content ]


container : List (Html msg) -> Html msg
container child =
    div
        [ css
            [ Bp.md
                [ Tw.px_24
                ]
            , Tw.flex
            , Tw.flex_col
            ]
        ]
        child


viewIf : Bool -> Html msg -> Html msg
viewIf condition content =
    if condition then
        content

    else
        text ""
