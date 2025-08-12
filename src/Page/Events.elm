module Page.Events exposing (Data, Model, Msg, page)

-- import MarkdownCodec

import CurrentDate
import Data.Photo exposing (Photo, photoJSONDecoder)
import DataSource exposing (DataSource)
import DataSource.Glob as Glob
import Head
import Head.Seo as Seo
import Css
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (css, src)
import MarkdownCodec
import OptimizedDecoder
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Path
import Shared
import Site exposing (commonSeo)
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
    , eventDate : String
    , eventTime : Maybe String
    , eventEndDate : Maybe String
    , eventEndTime : Maybe String
    , publishedFrom : Maybe String
    }


type alias Section =
    { metadata : Metadata
    , content : List (Html Msg)
    }


type alias Data =
    { sections : List Section
    , currentDate : String
    }


eventsGlob : DataSource (List String)
eventsGlob =
    Glob.succeed (\s -> s)
        |> Glob.match (Glob.literal "content/events/")
        |> Glob.capture Glob.wildcard
        |> Glob.match (Glob.literal ".md")
        |> Glob.toDataSource


data : DataSource Data
data =
    DataSource.map2 Data
        (eventsGlob
            |> DataSource.map
                (\contentList ->
                    List.map
                        (\content ->
                            MarkdownCodec.withFrontmatter Section
                                frontmatterDecoder
                                TailwindMarkdownRenderer.renderer
                                ("content/events/" ++ content ++ ".md")
                        )
                        contentList
                )
            |> DataSource.resolve
        )
        getCurrentDate


getCurrentDate : DataSource String
getCurrentDate =
    -- This returns the actual date when the site was built
    CurrentDate.currentDate


frontmatterDecoder : OptimizedDecoder.Decoder Metadata
frontmatterDecoder =
    OptimizedDecoder.map7 Metadata
        (OptimizedDecoder.field "title" OptimizedDecoder.string)
        (OptimizedDecoder.field "photo" photoJSONDecoder)
        (OptimizedDecoder.field "eventDate" OptimizedDecoder.string)
        (OptimizedDecoder.maybe (OptimizedDecoder.field "eventTime" OptimizedDecoder.string))
        (OptimizedDecoder.maybe (OptimizedDecoder.field "eventEndDate" OptimizedDecoder.string))
        (OptimizedDecoder.maybe (OptimizedDecoder.field "eventEndTime" OptimizedDecoder.string))
        (OptimizedDecoder.maybe (OptimizedDecoder.field "publishedFrom" OptimizedDecoder.string))


head :
    StaticPayload Data RouteParams
    -> List Head.Tag
head _ =
    Seo.summaryLarge
        { commonSeo
            | title = "Aspargesgården | Arrangementer"
        }
        |> Seo.website


view :
    Maybe PageUrl
    -> Shared.Model
    -> StaticPayload Data RouteParams
    -> View Msg
view _ _ static =
    let
        currentDate =
            static.data.currentDate

        publishedEvents =
            static.data.sections
                |> List.filter (\s -> isPublished currentDate s.metadata)
                |> List.sortBy (\s -> s.metadata.eventDate)

        ( upcomingEvents, pastEvents ) =
            publishedEvents
                |> List.partition (\s -> s.metadata.eventDate >= currentDate)

        -- Sort past events in reverse chronological order (newest first)
        sortedPastEvents =
            pastEvents
                |> List.sortBy (\s -> s.metadata.eventDate)
                |> List.reverse
    in
    View.html "Arrangementer"
        [ container
            [ div [] [ headline "Arrangementer" ] ]
        , if not (List.isEmpty upcomingEvents) then
            div
                [ css
                    [ Tw.w_full
                    , Tw.flex
                    , Tw.flex_col
                    ]
                ]
                (List.indexedMap section upcomingEvents)

          else
            container
                [ div
                    [ css
                        [ Tw.text_center
                        , Tw.py_12
                        , Tw.text_gray_600
                        ]
                    ]
                    [ p [] [ text "Ingen kommende arrangementer for øyeblikket. Se noen av våre tidligere arrangementer nedenfor for hva som kan komme i fremtiden!" ] ]
                ]
        , if not (List.isEmpty pastEvents) then
            div []
                [ container
                    [ div [ css [ Tw.mt_16 ] ] [ headline "Tidligere arrangementer" ] ]
                , div
                    [ css
                        [ Tw.w_full
                        , Tw.flex
                        , Tw.flex_col
                        ]
                    ]
                    (List.indexedMap section sortedPastEvents)
                ]

          else
            div [] []
        ]


section : Int -> Section -> Html Msg
section index sect =
    let
        dateTimeDisplay = formatEventDateTime sect.metadata
        
        contentWithDateTime =
            [ headline sect.metadata.title
            , p [ css [ Tw.pb_3, Tw.text_sort, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ] 
                [ strong [ css [ Tw.font_bold ] ] [ text "Dato: " ]
                , text dateTimeDisplay 
                ]
            ] ++ sect.content
        
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
                        , Tw.h_1over2
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
                    , Tw.px_12
                    , Tw.py_8
                    , Tw.w_full
                    ]
                ]
                contentWithDateTime
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


isPublished : String -> Metadata -> Bool
isPublished currentDate metadata =
    case metadata.publishedFrom of
        Nothing ->
            False

        Just publishDate ->
            -- Check if publishedFrom <= currentDate AND publishedFrom < eventDate
            (publishDate <= currentDate) && (publishDate < metadata.eventDate)


formatEventDateTime : Metadata -> String
formatEventDateTime metadata =
    let
        formatDate dateStr =
            case String.split "-" dateStr of
                [ year, month, day ] ->
                    day ++ ". " ++ monthToNorwegian month ++ " " ++ year

                _ ->
                    dateStr

        startDateFormatted =
            formatDate metadata.eventDate

        timeStr =
            case metadata.eventTime of
                Just time ->
                    " kl " ++ time

                Nothing ->
                    ""

        endDateStr =
            case metadata.eventEndDate of
                Just endDate ->
                    if endDate == metadata.eventDate then
                        -- Same day, just show end time if available
                        case metadata.eventEndTime of
                            Just endTime ->
                                " - " ++ endTime

                            Nothing ->
                                ""

                    else
                        -- Different days
                        " - "
                            ++ formatDate endDate
                            ++ (case metadata.eventEndTime of
                                    Just endTime ->
                                        " kl " ++ endTime

                                    Nothing ->
                                        ""
                               )

                Nothing ->
                    ""
    in
    startDateFormatted ++ timeStr ++ endDateStr ++ " på Aspargesgården"


monthToNorwegian : String -> String
monthToNorwegian month =
    case month of
        "01" ->
            "januar"

        "02" ->
            "februar"

        "03" ->
            "mars"

        "04" ->
            "april"

        "05" ->
            "mai"

        "06" ->
            "juni"

        "07" ->
            "juli"

        "08" ->
            "august"

        "09" ->
            "september"

        "10" ->
            "oktober"

        "11" ->
            "november"

        "12" ->
            "desember"

        _ ->
            month
