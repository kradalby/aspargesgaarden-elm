module View.Misc exposing
    ( contact
    , container
    , headline
    , imgWithPhotographer
    , paragraph
    , photographerLink
    , responsiveImg
    , viewIf
    )

import Css
import Css.Global
import Data.Photo exposing (Photographer)
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, class, css, href, rel, src, target, type_)
import String.Format
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
            [ img [ src "/ressurser/takontakt.png", alt "Ta kontakt" ] []
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
                , Css.hover
                    [ Css.Global.descendants
                        [ Css.Global.selector ".photographercredit"
                            [ Tw.opacity_100 ]
                        ]
                    ]
                ]
    in
    div []
        [ div
            (style
                :: attr
            )
            [ responsiveImg
                [ css imgStyles
                ]
                photoPath
            , div
                [ class "photographercredit"
                , css
                    [ Tw.opacity_0
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


imgSizes : List Int
imgSizes =
    [ 2048, 1536, 1280, 1024, 768, 640, 320 ]


imgTypes : List String
imgTypes =
    -- Order matters, the browser will try the first listed
    -- meaning that we will try to load them in order of "optimal".
    [ "avif", "webp", "jpeg" ]


responsiveSrcSet : String -> List Int -> String
responsiveSrcSet pathTemplate sizes =
    List.map
        (\size ->
            "/{{ path }} {{ size }}w"
                |> String.Format.namedValue "path" (String.Format.namedValue "size" (String.fromInt size) pathTemplate)
                |> String.Format.namedValue "size" (String.fromInt size)
        )
        sizes
        |> List.append
            -- TODO: Verify that we want the biggest image?
            [ String.Format.namedValue "size" "2048" pathTemplate
            ]
        |> String.join ", "


responsiveSource : String -> String -> Html msg
responsiveSource imageType basePath =
    let
        pathTemplate =
            "{{ basePath }}_{{ size }}w_resize.{{ extension }}"
                |> String.Format.namedValue "basePath" basePath
                |> String.Format.namedValue "extension" imageType

        mimeType =
            String.Format.namedValue "type" imageType "image/{{ type }}"

        sources =
            responsiveSrcSet pathTemplate imgSizes
    in
    source [ attribute "srcset" sources, type_ mimeType ] []


responsiveImg : List (Html.Styled.Attribute msg) -> String -> Html msg
responsiveImg attrs basePath =
    let
        sources =
            List.map (\imageType -> responsiveSource imageType basePath) imgTypes

        fallback =
            -- img (src (basePath ++ ".jpeg") :: attrs) []
            img attrs []
    in
    node "picture" attrs (sources ++ [ fallback ])
