module Page.Gallery exposing (Data, Model, Msg, page)

import Browser.Events
import Css
import Data.Photo exposing (Gallery, Photo, yamlFileToDataSourceGallery)
import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (css, src)
import Html.Styled.Events exposing (onClick)
import Json.Decode as Decode
import List.Extra
import Page exposing (StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)
import View.Misc exposing (container, headline, imgWithPhotographer, photographerLink, responsiveImg, viewIf)


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


type alias Data =
    Gallery


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
                \_ _ _ _ msg model ->
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
                \_ _ _ _ ->
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


data : DataSource Data
data =
    yamlFileToDataSourceGallery "content/gallery.yaml"


head :
    StaticPayload Data RouteParams
    -> List Head.Tag
head _ =
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
view _ _ model static =
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
    div
        [ css
            [ Bp.lg
                [ Tw.w_108
                ]
            , Bp.md
                [ Tw.w_80
                , Tw.mr_6
                , Tw.mb_6
                ]
            ]
        ]
        [ imgWithPhotographer [ onClick (ToggleSlideShow index) ] [] photo.path photo.photographer
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
                        [ responsiveImg [] photo.path
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
