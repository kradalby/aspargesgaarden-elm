module View.Misc exposing (contact, container, headline, imgWithPhotographer, paragraph, photographerLink, viewIf)

import Css
import Data.Photo exposing (Photographer)
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, css, href, rel, src, target)
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


photographerLink : Photographer -> String -> Html msg
photographerLink photographer prefix =
    case photographer.website of
        "" ->
            text <| prefix ++ " " ++ photographer.name

        website ->
            a
                [ href website
                , target "_blank"
                , rel "noopener noreferrer"
                ]
                [ text <| prefix ++ " " ++ photographer.name ]


imgWithPhotographer : List (Attribute msg) -> List Css.Style -> String -> Photographer -> Html msg
imgWithPhotographer attr imgStyles photoPath photographer =
    let
        style =
            css
                [ Tw.relative
                ]
    in
    div []
        [ div
            (style
                :: attr
            )
            [ img
                [ src
                    photoPath
                , css imgStyles
                ]
                []
            , div
                [ css
                    [ Css.hover
                        [ Tw.opacity_100
                        ]
                    , Tw.opacity_0
                    , Tw.absolute
                    , Tw.left_0
                    , Tw.bottom_0
                    , Tw.right_0
                    , Tw.z_10
                    , Tw.flex
                    , Tw.justify_around
                    , Tw.items_end
                    , Tw.bg_brown
                    , Tw.text_black
                    ]
                ]
                [ photographerLink photographer "av " ]
            ]
        ]
