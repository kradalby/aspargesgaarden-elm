module View.Header exposing (..)

import Css
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, css, for, href, id, src, type_)
import Path exposing (Path)
import Svg.Styled as SVG
import Svg.Styled.Attributes as SvgAttr
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw


view : Path -> Html msg
view currentPath =
    header
        [ css
            [ Tw.flex
            , Tw.flex_wrap
            , Tw.items_center
            , Bp.lg [ Tw.px_16, Tw.py_2 ]
            , Tw.px_4
            , Tw.py_0
            , Tw.border_b
            , Tw.border_brown_border
            ]
        ]
        [ div
            [ css
                [ Tw.flex
                , Tw.flex_1
                , Tw.justify_between
                , Tw.items_center
                ]
            ]
            [ a [ href "/" ]
                [ img
                    [ css [ Tw.h_28 ]
                    , src "/ikoner/logo.svg"
                    , alt "Aspargesgården"
                    ]
                    []
                ]
            ]
        , label
            [ for "menu-toggle"
            , css
                [ Bp.lg [ Tw.hidden ]
                , Tw.cursor_pointer
                , Tw.block
                ]
            ]
            [ SVG.svg
                [ SvgAttr.css
                    [ Tw.fill_current
                    , Tw.text_brown_link
                    ]

                -- , SvgAttr.xmlns "http://www.w3.org/2000/svg"
                , SvgAttr.width "20"
                , SvgAttr.height "20"
                , SvgAttr.viewBox "0 0 20 20"
                ]
                [ SVG.title []
                    [ text "menu" ]
                , SVG.path
                    [ SvgAttr.d "M0 3h20v2H0V3zm0 6h20v2H0V9zm0 6h20v2H0v-2z" ]
                    []
                ]
            ]
        , input
            [ css
                [ Bp.lg
                    [ Tw.flex
                    , Tw.items_center
                    , Tw.hidden
                    , Tw.w_auto
                    ]
                , Tw.hidden
                , Tw.w_full
                ]
            , type_ "checkbox"
            , id "menu-toggle"
            ]
            []
        , div
            [ css
                [ Bp.lg
                    [ Tw.flex
                    , Tw.items_center
                    , Tw.w_auto
                    ]
                , Tw.hidden
                , Tw.w_full
                ]
            , id "menu"
            ]
            [ nav []
                [ ul
                    [ css
                        [ Bp.lg [ Tw.flex ]
                        , Tw.items_center
                        , Tw.justify_between
                        , Tw.text_base
                        , Tw.text_brown_link
                        ]
                    , id "menu"
                    ]
                    -- , headerLink currentPath "stable" "Fjøset"
                    -- , headerLink currentPath "barn" "Låven"
                    [ headerLink currentPath "facilities" "Muligheter"
                    , headerLink currentPath "gallery" "Galleri"
                    , headerLink currentPath "about" "Om gården"
                    , headerLink currentPath "contact" "Kontakt"
                    ]
                ]
            ]
        ]


headerLink : Path -> String -> String -> Html msg
headerLink currentPagePath linkTo name =
    let
        isCurrentPath : Bool
        isCurrentPath =
            List.head (Path.toSegments currentPagePath) == Just linkTo
    in
    li
        [ css
            [ Bp.lg [ Tw.p_4, Tw.px_6 ]
            , Tw.py_3
            , Tw.px_0
            , Tw.block
            ]
        ]
        [ a
            [ href ("/" ++ linkTo)
            , attribute "elm-pages:prefetch" "true"
            , css <|
                [ Tw.font_medium
                , Tw.text_xl
                , Css.fontFamilies [ "rift-soft", "sans-serif" ]
                , Css.hover
                    [ Tw.text_brown_link_active
                    ]
                ]
                    ++ (if isCurrentPath then
                            [ Css.hover
                                [ Tw.text_brown_link
                                ]
                            ]

                        else
                            []
                       )
            ]
            [ text name ]
        ]
