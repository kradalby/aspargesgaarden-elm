module BookingTest exposing (suite)

import Booking exposing (Answer(..))
import Day
import Expect
import Set
import Test exposing (Test, describe, test)


ymd : Int -> Int -> Int -> Int
ymd =
    Day.fromYmd


{-| Today Wednesday 11 November 2026; the calendar goes to September 2028.
Every Saturday of June 2027 is taken, and 29 May.
-}
cal : Booking.Calendar
cal =
    { today = ymd 2026 11 11
    , last = ymd 2028 9 30
    , taken = Set.fromList [ ymd 2027 5 29, ymd 2027 6 5, ymd 2027 6 12, ymd 2027 6 19, ymd 2027 6 26, ymd 2026 11 14 ]
    }


suite : Test
suite =
    describe "Booking"
        [ describe "Day"
            [ test "round trip" <| \_ -> Day.fromIso "2027-06-12" |> Maybe.map Day.toIso |> Expect.equal (Just "2027-06-12")
            , test "epoch is a Thursday" <| \_ -> Day.weekday 0 |> Expect.equal 3
            , test "a Saturday" <| \_ -> Day.weekday (ymd 2027 6 12) |> Expect.equal 5
            , test "leap day" <| \_ -> Day.fromIso "2028-02-29" |> Maybe.map Day.toIso |> Expect.equal (Just "2028-02-29")
            , test "no 29 February 2027" <| \_ -> Day.fromIso "2027-02-29" |> Expect.equal Nothing
            , test "junk" <| \_ -> List.filterMap Day.fromIso [ "", "2027-6-12", "12.06.2027", "2027-13-01", "abcd-ef-gh" ] |> Expect.equal []
            , test "months across a year" <| \_ -> Day.addMonths 2 (ymd 2026 11 30) |> Expect.equal (ymd 2027 1 1)
            , test "months back" <| \_ -> Day.addMonths -1 (ymd 2027 1 15) |> Expect.equal (ymd 2026 12 1)
            ]
        , describe "window"
            [ test "not before this month" <| \_ -> Booking.clampStart cal (ymd 2026 10 1) |> Expect.equal (ymd 2026 11 1)
            , test "the last month is the third shown" <| \_ -> Booking.clampStart cal (ymd 2028 9 1) |> Expect.equal (ymd 2028 7 1)
            , test "the asked month comes first" <| \_ -> Booking.startAt cal (ymd 2027 6 12) |> Expect.equal (ymd 2027 6 1)
            , test "near the end the asked month is as near first as it can be" <| \_ -> Booking.startAt cal (ymd 2028 9 2) |> Expect.equal (ymd 2028 7 1)
            ]
        , describe "answer"
            [ test "past" <| \_ -> Booking.answer cal (ymd 2026 11 10) |> Expect.equal Past
            , test "beyond the calendar" <| \_ -> Booking.answer cal (ymd 2028 10 1) |> Expect.equal Beyond
            , test "free" <| \_ -> Booking.answer cal (ymd 2027 7 3) |> Expect.equal Free
            , test "taken today is taken, and nothing before today is offered" <|
                \_ ->
                    Booking.answer { cal | today = ymd 2026 11 14 } (ymd 2026 11 14)
                        |> Expect.equal (Taken [ ymd 2026 11 21, ymd 2026 11 28 ])
            , test "taken: two free Saturdays either side, skipping taken ones" <|
                \_ ->
                    Booking.answer cal (ymd 2027 6 12)
                        |> Expect.equal (Taken [ ymd 2027 5 15, ymd 2027 5 22, ymd 2027 7 3, ymd 2027 7 10 ])
            , test "suggestions stop at today" <|
                \_ ->
                    Booking.nearestFree cal (ymd 2026 11 14)
                        |> Expect.equal [ ymd 2026 11 21, ymd 2026 11 28 ]
            , test "suggestions stop at the last day" <|
                \_ ->
                    Booking.nearestFree { cal | taken = Set.insert (ymd 2028 9 30) cal.taken } (ymd 2028 9 30)
                        |> Expect.equal [ ymd 2028 9 16, ymd 2028 9 23 ]
            ]
        , describe "words"
            [ test "long date" <| \_ -> Booking.longDate (ymd 2027 6 12) |> Expect.equal "Lørdag 12. juni 2027"
            , test "plural" <| \_ -> Booking.weekdayPlural (ymd 2027 6 12) |> Expect.equal "lørdager"
            , test "range in a year" <| \_ -> Booking.monthRange (ymd 2027 4 1) (ymd 2027 6 1) |> Expect.equal "April – juni 2027"
            , test "range across years" <| \_ -> Booking.monthRange (ymd 2026 11 1) (ymd 2027 1 1) |> Expect.equal "November 2026 – januar 2027"
            , test "one month" <| \_ -> Booking.monthRange (ymd 2027 4 1) (ymd 2027 4 1) |> Expect.equal "April 2027"
            , test "edge" <| \_ -> Booking.edge cal |> Expect.equal "Kalenderen går til september 2028. Spør oss om datoer etter det."
            ]
        ]
