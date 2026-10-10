module View.Event exposing (bulletList, callout, content, detail, introduction, paragraph, section)

import Css
import Html.Styled exposing (Html, div, h2, h3, li, p, strong, text, ul)
import Html.Styled.Attributes exposing (css)
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View.Misc exposing (container)


content : List (Html msg) -> Html msg
content children =
    container
        [ div
            [ css
                [ Tw.max_w_4xl
                , Tw.mx_auto
                , Tw.w_full
                , Tw.py_8
                , Tw.px_4
                , Bp.md [ Tw.px_0 ]
                , Tw.text_sort
                , Css.fontFamilies [ "Avenir", "sans-serif" ]
                ]
            ]
            children
        ]


introduction : String -> Html msg
introduction title =
    h2
        [ css
            [ Tw.text_4xl
            , Tw.mb_6
            , Tw.text_tre
            , Css.fontFamilies [ "Cardillac", "cursive" ]
            ]
        ]
        [ text title ]


paragraph : String -> Html msg
paragraph body =
    p [ css [ Tw.pb_3 ] ] [ text body ]


section : String -> List (Html msg) -> Html msg
section title children =
    div [ css [ Tw.my_8 ] ]
        (h3
            [ css
                [ Tw.text_3xl
                , Tw.mb_4
                , Tw.text_tre
                , Css.fontFamilies [ "Cardillac", "cursive" ]
                ]
            ]
            [ text title ]
            :: children
        )


detail : String -> String -> Html msg
detail label value =
    p [ css [ Tw.pb_3 ] ]
        [ strong [ css [ Tw.font_bold ] ] [ text (label ++ ": ") ]
        , text value
        ]


bulletList : List String -> Html msg
bulletList items =
    ul [ css [ Tw.list_disc, Tw.list_inside, Tw.space_y_2 ] ]
        (List.map (\item -> li [] [ text item ]) items)


callout : String -> List (Html msg) -> Html msg
callout title children =
    div
        [ css
            [ Tw.bg_brown
            , Tw.bg_opacity_20
            , Tw.p_6
            , Tw.my_8
            ]
        ]
        (p [ css [ Tw.text_lg, Tw.font_bold, Tw.mb_2 ] ] [ text title ]
            :: children
        )
