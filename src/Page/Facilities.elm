module Page.Facilities exposing (Data, Model, Msg, page)

-- import MarkdownCodec

import Data.Photo exposing (Photo, photoJSONDecoder)
import DataSource exposing (DataSource)
import DataSource.Glob as Glob
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (css, src)
import MarkdownCodec
import OptimizedDecoder
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import TailwindMarkdownRenderer
import View exposing (View)
import View.Misc exposing (container, headline, imgWithPhotographer)


type alias Model =
    ()


type alias Msg =
    Never


type alias RouteParams =
    {}


page : Page RouteParams Data
page =
    Page.single
        { head = head
        , data = data
        }
        |> Page.buildNoState { view = view }


type alias Metadata =
    { title : String
    , photo : Photo
    }


type alias Section =
    { metadata : Metadata
    , content : List (Html Msg)
    }


type alias Data =
    List Section


facilitiesGlob : DataSource (List String)
facilitiesGlob =
    Glob.succeed (\s -> s)
        |> Glob.match (Glob.literal "content/facilities/")
        |> Glob.capture Glob.wildcard
        |> Glob.match (Glob.literal ".md")
        |> Glob.toDataSource


data : DataSource Data
data =
    facilitiesGlob
        |> DataSource.map
            (\contentList ->
                List.map
                    (\content ->
                        MarkdownCodec.withFrontmatter Section
                            frontmatterDecoder
                            TailwindMarkdownRenderer.renderer
                            ("content/facilities/" ++ content ++ ".md")
                    )
                    contentList
            )
        |> DataSource.resolve


frontmatterDecoder : OptimizedDecoder.Decoder Metadata
frontmatterDecoder =
    OptimizedDecoder.map2 Metadata
        (OptimizedDecoder.field "title" OptimizedDecoder.string)
        (OptimizedDecoder.field "photo" photoJSONDecoder)


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
    -> StaticPayload Data RouteParams
    -> View Msg
view _ _ static =
    View.html "Muligheter"
        [ container
            [ div [] [ headline "Muligheter" ] ]
        , div
            [ css
                [ Tw.w_full
                , Tw.flex
                , Tw.flex_col
                ]
            ]
          <|
            List.indexedMap section static.data
        ]


section : Int -> Section -> Html Msg
section index sect =
    let
        elements =
            [ div
                [ css <|
                    [ Bp.lg
                        [ Tw.w_1over3
                        ]
                    , Bp.md
                        [ Tw.w_1over2
                        ]
                    , Tw.w_full
                    ]
                ]
                [ div
                    [ css <|
                        [ Bp.lg
                            [ Tw.h_auto
                            , Tw.w_full
                            ]
                        , Bp.md
                            [ Tw.h_auto
                            ]
                        , Tw.w_full
                        ]
                            ++ (if isEven index then
                                    []

                                else
                                    [ Tw.float_right ]
                               )
                    ]
                    [ imgWithPhotographer []
                        [ Bp.md
                            [ Tw.h_auto
                            ]
                        , Tw.object_cover
                        , Tw.h_48
                        , Tw.w_full
                        ]
                        sect.metadata.photo.path
                        sect.metadata.photo.photographer
                    ]
                ]
            , div
                [ css <|
                    [ Bp.lg
                        [ Tw.w_2over3
                        ]
                    , Bp.md
                        [ Tw.w_1over2
                        ]
                    , Tw.h_full
                    , Tw.p_12
                    , Tw.w_full
                    ]
                ]
                sect.content
            ]
    in
    div
        [ css <|
            [ Tw.w_full
            , Tw.flex
            , Tw.flex_wrap
            ]
                ++ (if isEven index then
                        [ Bp.md [ Tw.pl_12 ], Tw.bg_brown, Tw.bg_opacity_20, Tw.flex_row_reverse ]

                    else
                        [ Tw.flex_row ]
                   )
        ]
        elements


isEven : Int -> Bool
isEven num =
    case modBy 2 num of
        1 ->
            False

        _ ->
            True
