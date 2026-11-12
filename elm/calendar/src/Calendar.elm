port module Calendar exposing (main)

{-| The contact page's booking calendar. The server renders three months
complete; this takes that part over (ui.BookingCalendar's data-calendar),
draws the same three, and adds paging a month at a time and the question
"is my date free?". The taken days come in the page: nothing is fetched.
-}

import Booking exposing (Answer(..), Calendar)
import Browser
import Day exposing (Day)
import Html exposing (Html, a, button, div, input, label, li, p, span, table, tbody, td, text, th, thead, tr, ul)
import Html.Attributes exposing (attribute, class, for, href, id, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as Decode exposing (Decoder)
import Set


{-| The browser's clock, sent every minute and when the tab comes back into
view: its day in Oslo, and its milliseconds. A later day moves today on (a
tab left open over midnight); a wrong clock can never take it back before
the server's day. The milliseconds age the tab (Booking.expired).
-}
port tick : ({ today : String, now : Float } -> msg) -> Sub msg


{-| The tab is too old to show bookings: boot.js removes the server's
weekend list too.
-}
port expired : () -> Cmd msg


{-| An answer is drawn: boot.js scrolls it into view if it is not.
-}
port answered : () -> Cmd msg


type alias Model =
    { cal : Calendar
    , phone : String
    , tel : String
    , start : Day
    , input : String
    , asked : Maybe Day
    , chosen : Maybe Day
    , age : Booking.Age
    , stale : Bool
    }


type Msg
    = Previous
    | Next
    | Input String
    | Ask
    | Pick Day
    | Tick { today : String, now : Float }


main : Program Decode.Value (Maybe Model) Msg
main =
    Browser.element
        { init = \flags -> ( Result.toMaybe (Decode.decodeValue decoder flags), Cmd.none )
        , update =
            \msg maybe ->
                case maybe of
                    Just model ->
                        update msg model |> Tuple.mapFirst Just

                    Nothing ->
                        ( Nothing, Cmd.none )
        , view = Maybe.map view >> Maybe.withDefault (text "")
        , subscriptions = \_ -> tick Tick
        }


decoder : Decoder Model
decoder =
    let
        day =
            Decode.string
                |> Decode.andThen
                    (\s ->
                        case Day.fromIso s of
                            Just d ->
                                Decode.succeed d

                            Nothing ->
                                Decode.fail ("not a date: " ++ s)
                    )

        model t l taken phone tel maxAge loaded =
            { cal = { today = t, last = l, taken = Set.fromList taken }
            , phone = phone
            , tel = tel
            , start = Day.monthStart t
            , input = ""
            , asked = Nothing
            , chosen = Nothing
            , age = { loaded = loaded, maxAge = maxAge }
            , stale = False
            }
    in
    Decode.map7 model
        (Decode.field "today" day)
        (Decode.field "last" day)
        (Decode.field "taken" (Decode.list day))
        (Decode.field "phone" Decode.string)
        (Decode.field "tel" Decode.string)
        (Decode.field "maxAge" Decode.int)
        (Decode.field "loaded" Decode.float)


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    let
        cal =
            model.cal
    in
    case msg of
        Previous ->
            ( { model | start = Booking.clampStart cal (Day.addMonths -1 model.start) }, Cmd.none )

        Next ->
            ( { model | start = Booking.clampStart cal (Day.addMonths 1 model.start) }, Cmd.none )

        Input s ->
            ( { model | input = s }, Cmd.none )

        Ask ->
            case Day.fromIso model.input of
                Just d ->
                    ( { model | asked = Just d, chosen = Just d, start = Booking.startAt cal d }, answered () )

                Nothing ->
                    ( model, Cmd.none )

        Pick d ->
            ( { model | chosen = Just d, start = Booking.startAt cal d }, Cmd.none )

        Tick t ->
            if model.stale then
                ( model, Cmd.none )

            else if Booking.expired model.age t.now then
                ( { model | stale = True }, expired () )

            else
                case Day.fromIso t.today of
                    Just d ->
                        if d > cal.today then
                            let
                                moved =
                                    { cal | today = d }
                            in
                            ( { model | cal = moved, start = Booking.clampStart moved model.start }, Cmd.none )

                        else
                            ( model, Cmd.none )

                    Nothing ->
                        ( model, Cmd.none )



-- VIEW: the server's markup (ui/bookingcalendar.go), class for class, with
-- the question in the box the server leaves for it and the paging buttons.


{-| data-booted tells boot.js the flags were read and this may replace the
server's months. Scroll anchoring is off inside: a browser keeping the
months still while an answer grows above them would scroll the answer away.
-}
view : Model -> Html Msg
view model =
    let
        cal =
            model.cal

        end =
            Day.addMonths (Booking.shown - 1) model.start
    in
    if model.stale then
        div [ attribute "data-booted" "" ]
            [ p [ class "max-w-2xl text-lg text-sort", attribute "role" "status" ]
                [ text "Kalenderen er ikke tilgjengelig akkurat nå. Last inn siden på nytt, eller ring "
                , a [ href model.tel, class "underline whitespace-nowrap" ] [ text model.phone ]
                , text " for å spørre om en dato."
                ]
            ]

    else
        div [ attribute "data-booted" "", class "[overflow-anchor:none]" ]
            [ question model
            , legend
            , div [ class "flex items-center justify-between gap-2 h-12 mb-2" ]
                [ p [ class "font-bold text-sort", attribute "aria-live" "polite" ] [ text (Booking.monthRange model.start end) ]
                , div [ class "flex gap-2" ]
                    [ pageButton Previous (model.start == Booking.clampStart cal (Day.addMonths -1 model.start)) "‹" "Forrige" True
                    , pageButton Next (model.start == Booking.clampStart cal (Day.addMonths 1 model.start)) "›" "Neste" False
                    ]
                ]
            , div [ class "grid items-start gap-x-10 gap-y-8 sm:grid-cols-2 lg:grid-cols-3" ]
                (List.map (\i -> monthTable model (Day.addMonths i model.start)) (List.range 0 (Booking.shown - 1)))
            , p [ class "pt-6 text-sort" ] [ text (Booking.edge cal) ]
            ]


legend : Html msg
legend =
    let
        item swatch words =
            li [ class "flex items-center gap-2" ]
                [ span [ class ("inline-block size-5 " ++ swatch), attribute "aria-hidden" "true" ] [], text words ]
    in
    ul [ class "flex flex-wrap gap-x-6 gap-y-2 pb-4 text-sm text-sort" ]
        [ item "bg-rust" "Opptatt"
        , item "border border-brown-border/50" "Ledig"
        , item "outline-2 -outline-offset-2 outline-sort" "I dag"
        ]


{-| aria-disabled, not disabled: a disabled button drops the focus a
keyboard user had on it. On a phone only the arrow shows, so the months'
names keep their one line beside the buttons.
-}
pageButton : Msg -> Bool -> String -> String -> Bool -> Html Msg
pageButton msg atEnd arrow words arrowFirst =
    let
        parts =
            [ span [ attribute "aria-hidden" "true" ] [ text arrow ]
            , span [ class "hidden sm:inline" ] [ text words ]
            ]
    in
    button
        ([ type_ "button"
         , attribute "aria-label" (words ++ " måned")
         , class "flex gap-2 py-2 px-3 border border-brown-border text-sm font-bold uppercase tracking-wide text-sort hover:bg-brown/20 aria-disabled:opacity-40 aria-disabled:hover:bg-transparent aria-disabled:cursor-default"
         ]
            ++ (if atEnd then
                    [ attribute "aria-disabled" "true" ]

                else
                    [ onClick msg ]
               )
        )
        (if arrowFirst then
            parts

         else
            List.reverse parts
        )


weekdayHeads : List ( String, String )
weekdayHeads =
    [ ( "ma", "mandag" ), ( "ti", "tirsdag" ), ( "on", "onsdag" ), ( "to", "torsdag" ), ( "fr", "fredag" ), ( "lø", "lørdag" ), ( "sø", "søndag" ) ]


monthTable : Model -> Day -> Html msg
monthTable model first =
    let
        next =
            Day.addMonths 1 first

        lead =
            Day.weekday first

        cells =
            List.repeat lead Nothing
                ++ List.map Just (List.range first (next - 1))

        weeks list =
            if List.isEmpty list then
                []

            else
                List.take 7 list :: weeks (List.drop 7 list)

        pad week =
            week ++ List.repeat (7 - List.length week) Nothing
    in
    table [ class "w-full table-fixed border-separate border-spacing-1 p-2 border border-brown-border/50" ]
        [ Html.caption [ class "pb-1 text-left font-display text-2xl text-tre" ] [ text (Booking.monthRange first first) ]
        , thead [] [ tr [] (List.map (\( short, long ) -> th [ attribute "scope" "col", attribute "abbr" long, class "pb-1 text-xs font-bold uppercase text-gray-600" ] [ text short ]) weekdayHeads) ]
        , tbody [] (List.map (\week -> tr [] (List.map (cell model) (pad week))) (weeks cells))
        ]


cell : Model -> Maybe Day -> Html msg
cell model maybe =
    case maybe of
        Nothing ->
            td [] []

        Just d ->
            let
                past =
                    d < model.cal.today

                taken =
                    Set.member d model.cal.taken && not past

                isToday =
                    d == model.cal.today

                chosen =
                    model.chosen == Just d

                classes =
                    [ ( "h-9 text-center text-sm tabular-nums", True )
                    , ( "bg-rust font-bold text-white", taken )
                    , ( "text-gray-500", past )
                    , ( "text-sort", not taken && not past )
                    , ( "font-bold outline-2 -outline-offset-2 outline-sort", isToday )
                    , ( chosenClass, chosen )
                    ]
                        |> List.filter Tuple.second
                        |> List.map Tuple.first
                        |> String.join " "

                words =
                    [ ( " opptatt", taken ), ( " i dag", isToday ), ( " din dato", chosen ) ]
                        |> List.filter Tuple.second
                        |> List.map (\( w, _ ) -> span [ class "sr-only" ] [ text w ])
            in
            td
                (class classes
                    :: (if isToday then
                            [ attribute "aria-current" "date" ]

                        else
                            []
                       )
                )
                (text (String.fromInt (Day.day d)) :: words)


{-| The asked day is underlined in its cell's own colour: white on a taken
day, dark on a free one, and unlike today's frame.
-}
chosenClass : String
chosenClass =
    "font-bold underline decoration-2 underline-offset-4"


question : Model -> Html Msg
question model =
    div [ class "mb-8 p-6 bg-brown/20" ]
        [ Html.form [ onSubmit Ask, class "grid grid-rows-[1.5rem_3rem] gap-1" ]
            [ label [ for "kalender-dato", class "leading-6 truncate font-bold text-sort" ] [ text "Hvilken dato tenker du på?" ]
            , div [ class "flex gap-4" ]
                [ input
                    [ type_ "date"
                    , id "kalender-dato"
                    , value model.input
                    , onInput Input
                    , class "h-12 w-48 px-3 bg-white border border-brown-border text-lg text-sort"
                    ]
                    []
                , button [ type_ "submit", class "h-12 px-6 bg-mose text-white text-sm font-bold uppercase tracking-wide hover:bg-mork-eukalyptus" ] [ text "Sjekk" ]
                ]
            ]
        , div [ id "kalender-svar", attribute "aria-live" "polite" ] (Maybe.map (answerView model) model.asked |> Maybe.withDefault [])
        ]


answerView : Model -> Day -> List (Html Msg)
answerView model d =
    let
        sentence mark words =
            p [ class ("mt-6 pl-4 border-l-4 text-2xl text-sort " ++ mark) ] [ text words ]

        call before after =
            p [ class "pt-3 text-sort" ] [ text before, a [ href model.tel, class "underline whitespace-nowrap" ] [ text model.phone ], text after ]

        marked =
            p [ class "pt-3 text-sm text-sort" ]
                [ span [ class ("inline-block w-6 text-center " ++ chosenClass), attribute "aria-hidden" "true" ] [ text (String.fromInt (Day.day (Maybe.withDefault d model.chosen))) ]
                , text " Din dato er understreket i kalenderen."
                ]
    in
    case Booking.answer model.cal d of
        Past ->
            [ sentence "border-brown-border" (Booking.longDate d ++ " har vært.") ]

        Beyond ->
            [ sentence "border-brown-border" (Booking.longDate d ++ " er lenger frem enn kalenderen går.")
            , call "Ring " " for å spørre om den."
            ]

        Free ->
            [ sentence "border-mose" (Booking.longDate d ++ " er ledig.")
            , call "Ring " " for å leie."
            , marked
            ]

        Taken [] ->
            [ sentence "border-rust" (Booking.longDate d ++ " er opptatt.")
            , call ("Ingen ledige " ++ Booking.weekdayPlural d ++ " i nærheten. Ring ") " for andre datoer."
            , marked
            ]

        Taken near ->
            [ sentence "border-rust" (Booking.longDate d ++ " er opptatt.")
            , p [ class "pt-4 pb-2 font-bold text-sort" ] [ text ("Nærmeste ledige " ++ Booking.weekdayPlural d ++ ":") ]
            , ul [ class "flex flex-wrap gap-2" ] (List.map (suggestion model) near)
            , marked
            ]


suggestion : Model -> Day -> Html Msg
suggestion model d =
    li []
        [ button
            [ type_ "button"
            , onClick (Pick d)
            , attribute "aria-pressed"
                (if model.chosen == Just d then
                    "true"

                 else
                    "false"
                )
            , class "py-2 px-3 border border-brown-border text-sort hover:bg-brown/20 aria-pressed:bg-mose aria-pressed:border-mose aria-pressed:text-white"
            ]
            [ text (Booking.longDate d) ]
        ]
