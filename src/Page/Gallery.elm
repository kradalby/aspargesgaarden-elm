module Page.Gallery exposing (Data, Model, Msg, page)

import Browser.Events
import Browser.Navigation
import Css
import DataSource exposing (DataSource)
import DataSource.File as File
import Dict
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, css, for, href, id, src, type_)
import Html.Styled.Events exposing (onClick, onInput)
import Json.Decode as Decode
import List.Extra
import Page exposing (Page, PageWithState, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)
import View.Misc exposing (contact, container, headline, paragraph, viewIf)
import Yaml.Decode as Y


type Direction
    = Left
    | Right


keyDecoder : Decode.Decoder (Maybe Direction)
keyDecoder =
    Decode.map toDirection (Decode.field "key" Decode.string)


toDirection : String -> Maybe Direction
toDirection string =
    case string of
        "ArrowLeft" ->
            Just Left

        "ArrowRight" ->
            Just Right

        _ ->
            Nothing


type alias Model =
    { slide : Int
    , showSlide : Bool
    , gallery : Gallery
    }


type Msg
    = OnKeyPress (Maybe Direction)
    | ToggleSlideShow Int
    | PreviousSlide
    | NextSlide


type alias RouteParams =
    {}


page : Page.PageWithState RouteParams Data Model Msg
page =
    Page.single
        { head = head
        , data = data
        }
        |> Page.buildWithLocalState
            { view = view
            , init = \_ _ staticPayload -> ( { slide = 0, showSlide = False, gallery = staticPayload.data }, Cmd.none )
            , update =
                \_ maybeNavigationKey sharedModel static msg model ->
                    case msg of
                        OnKeyPress (Just direction) ->
                            case direction of
                                Right ->
                                    ( { model | slide = nextSlideIndex model.gallery.photos model.slide }, Cmd.none )

                                Left ->
                                    ( { model | slide = previousSlideIndex model.gallery.photos model.slide }, Cmd.none )

                        ToggleSlideShow slide ->
                            ( { model
                                | showSlide = not model.showSlide
                                , slide = slide
                              }
                            , Cmd.none
                            )

                        PreviousSlide ->
                            ( { model | slide = previousSlideIndex model.gallery.photos model.slide }, Cmd.none )

                        NextSlide ->
                            ( { model | slide = nextSlideIndex model.gallery.photos model.slide }, Cmd.none )

                        _ ->
                            ( model, Cmd.none )
            , subscriptions =
                \maybePageUrl routeParams path model ->
                    Browser.Events.onKeyDown keyDecoder |> Sub.map OnKeyPress
            }


nextSlideIndex : List a -> Int -> Int
nextSlideIndex slides current =
    let
        length =
            List.length slides

        next =
            current + 1
    in
    if next < length then
        next

    else
        0


previousSlideIndex : List a -> Int -> Int
previousSlideIndex slides current =
    let
        length =
            List.length slides

        previous =
            current - 1
    in
    if previous < 0 then
        length - 1

    else
        previous


type alias Photographer =
    { name : String
    , website : String
    }


type alias PhotoYAML =
    { path : String
    , photographer : String
    , description : String
    }


type alias Photo =
    { path : String
    , photographer : Photographer
    , description : String
    }


type alias GalleryYAML =
    { photos : List PhotoYAML
    }


type alias Gallery =
    { photos : List Photo
    }


type alias Data =
    Gallery


photographersDecoder : Y.Decoder (Dict.Dict String Photographer)
photographersDecoder =
    Y.field "photographers" (Y.dict photographerDecoder)


photographerDecoder : Y.Decoder Photographer
photographerDecoder =
    Y.map2 Photographer
        (Y.field "name" Y.string)
        (Y.field "website" Y.string)


photoDecoder : Y.Decoder PhotoYAML
photoDecoder =
    Y.map3 PhotoYAML
        (Y.field "path" Y.string)
        (Y.field "photographer" Y.string)
        (Y.field "description" Y.string)


yamlToPhoto : Dict.Dict String Photographer -> PhotoYAML -> Photo
yamlToPhoto photographers photo =
    { path = photo.path
    , photographer = Dict.get photo.photographer photographers |> Maybe.withDefault { name = "Ukjent", website = "" }
    , description = photo.description
    }


data : DataSource Data
data =
    File.rawFile
        "content/gallery.yaml"
        |> DataSource.map
            (\content ->
                Y.fromString
                    photographersDecoder
                    content
                    |> Result.withDefault Dict.empty
            )
        |> DataSource.andThen
            (\photographers ->
                File.rawFile
                    "content/gallery.yaml"
                    |> DataSource.map
                        (\content ->
                            Y.fromString
                                (Y.map GalleryYAML
                                    (Y.field "photos" (Y.list photoDecoder))
                                )
                                content
                                |> Result.withDefault { photos = [] }
                        )
                    |> DataSource.map
                        (\gallery ->
                            let
                                photos =
                                    List.map (yamlToPhoto photographers) gallery.photos
                            in
                            { photos = photos }
                        )
            )


head :
    StaticPayload Data RouteParams
    -> List Head.Tag
head static =
    Seo.summary
        { canonicalUrlOverride = Nothing
        , siteName = "elm-pages"
        , image =
            { url = Pages.Url.external "TODO"
            , alt = "elm-pages logo"
            , dimensions = Nothing
            , mimeType = Nothing
            }
        , description = "TODO"
        , locale = Nothing
        , title = "TODO title" -- metadata.title -- TODO
        }
        |> Seo.website


view :
    Maybe PageUrl
    -> Shared.Model
    -> Model
    -> StaticPayload Data RouteParams
    -> View Msg
view maybeUrl sharedModel model static =
    View.html "Galleri"
        [ container
            [ viewIf model.showSlide (slideShow static.data model.slide)
            , div [] [ headline "Galleri" ]
            , div
                [ css
                    [ Tw.w_full
                    , Tw.flex
                    , Tw.flex_row
                    , Tw.flex_wrap
                    ]
                ]
              <|
                List.indexedMap
                    thumbnail
                    static.data.photos
            ]
        ]


thumbnail : Int -> Photo -> Html Msg
thumbnail index photo =
    div []
        [ div
            [ css
                [ Bp.lg
                    [ Tw.w_108
                    ]
                , Bp.md
                    [ Tw.w_80
                    , Tw.mr_6
                    , Tw.mb_6
                    ]
                , Tw.relative
                ]
            , onClick <| ToggleSlideShow index
            ]
            [ img
                [ src
                    photo.path
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
                [ photographerLink photo.photographer "av " ]
            ]
        ]


slideShow : Gallery -> Int -> Html Msg
slideShow gallery index =
    let
        maybePhoto =
            List.Extra.getAt index gallery.photos
    in
    case maybePhoto of
        Just photo ->
            div
                [ css
                    [ -- TODO: why does this not work?
                      Tw.duration_100
                    , Tw.ease_in_out
                    , Tw.z_20
                    ]
                ]
                [ div
                    [ css
                        [ Tw.w_full
                        , Tw.h_full
                        , Tw.bg_brown
                        , Tw.fixed
                        , Tw.top_0
                        , Tw.left_0
                        , Tw.opacity_60
                        ]
                    , onClick <| ToggleSlideShow 0
                    ]
                    []
                , div
                    [ css
                        [ Bp.xl
                            [ Tw.left_2
                            , Tw.right_2
                            , Tw.bottom_2
                            ]
                        , Tw.fixed
                        , Tw.top_0
                        , Tw.left_0
                        , Tw.flex
                        , Tw.flex_row
                        , Tw.justify_center
                        , Tw.h_full
                        ]
                    ]
                    [ div
                        [ css
                            [ Bp.xl
                                [ Tw.max_w_screen_lg
                                ]
                            , Bp.md
                                [ Tw.w_4over5
                                ]
                            , Tw.m_auto
                            , Tw.bg_white
                            , Tw.p_4
                            , Tw.pb_10
                            , Tw.static
                            ]
                        ]
                        [ img [ src photo.path ] []
                        , div
                            [ css
                                [ Tw.text_right
                                ]
                            ]
                            [ photographerLink photo.photographer "Fotograf:" ]
                        , div [] [ text photo.description ]
                        , closeSlideShow
                        , previousArrow
                        , nextArrow
                        ]
                    ]
                ]

        Nothing ->
            text ""


arrowDivStyle : List Css.Style
arrowDivStyle =
    [ Tw.w_14
    , Tw.h_14
    , Tw.text_4xl
    , Tw.text_brown_text
    , Tw.rounded_full
    , Tw.bg_white
    , Tw.absolute
    , Tw.flex
    , Tw.flex_col
    , Tw.justify_center
    , Tw.text_center
    ]


closeSlideShow : Html Msg
closeSlideShow =
    div
        [ css <|
            [ Tw.top_2
            , Tw.right_2
            ]
                ++ arrowDivStyle
        , onClick <| ToggleSlideShow 0
        ]
        [ text "X"
        ]


previousArrow : Html Msg
previousArrow =
    div
        [ css <|
            [ Bp.md
                [ Tw.top_1over2
                ]
            , Tw.left_2
            , Tw.bottom_6
            , Tw.pb_1
            , Tw.pr_1
            ]
                ++ arrowDivStyle
        , onClick PreviousSlide
        ]
        [ text "❮" ]


nextArrow : Html Msg
nextArrow =
    div
        [ css <|
            [ Bp.md
                [ Tw.top_1over2
                ]
            , Tw.right_2
            , Tw.bottom_6
            , Tw.pb_1
            , Tw.pl_1
            ]
                ++ arrowDivStyle
        , onClick NextSlide
        ]
        [ text "❯" ]


photographerLink : Photographer -> String -> Html msg
photographerLink photographer prefix =
    case photographer.website of
        "" ->
            text <| prefix ++ " " ++ photographer.name

        website ->
            a [ href website ] [ text <| prefix ++ " " ++ photographer.name ]
