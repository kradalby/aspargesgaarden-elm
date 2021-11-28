module View.Misc exposing (contact, headline, paragraph)

import Css
import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, css, for, href, id, src, type_)
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)


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
                , Tw.pl_24
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
