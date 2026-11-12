module Booking exposing
    ( Answer(..)
    , Calendar
    , answer
    , clampStart
    , edge
    , longDate
    , monthName
    , monthRange
    , nearestFree
    , shown
    , startAt
    , weekdayPlural
    )

{-| What the calendar knows and says, apart from how it looks: the window of
three months, the answer to "is my date free?", and the Norwegian words.
The server's booking package says the same in Go; the two must agree, or
the page changes when the app boots.
-}

import Day exposing (Day)
import Set exposing (Set)


type alias Calendar =
    { today : Day
    , last : Day
    , taken : Set Day
    }


{-| Months the window shows at once.
-}
shown : Int
shown =
    3


{-| The window's first month kept inside today's month and the last three.
-}
clampStart : Calendar -> Day -> Day
clampStart cal start =
    let
        first =
            Day.monthStart cal.today

        latest =
            Day.addMonths (1 - shown) cal.last
    in
    if Day.monthsBetween start latest < 0 then
        latest

    else if Day.monthsBetween first start < 0 then
        first

    else
        start


{-| The window with d's month first, or as near first as the calendar's
end allows: under the answer, the asked month is the one in view.
-}
startAt : Calendar -> Day -> Day
startAt cal d =
    clampStart cal (Day.monthStart d)


type Answer
    = Past
    | Beyond
    | Free
    | Taken (List Day)


answer : Calendar -> Day -> Answer
answer cal d =
    if d < cal.today then
        Past

    else if d > cal.last then
        Beyond

    else if Set.member d cal.taken then
        Taken (nearestFree cal d)

    else
        Free


{-| Up to two free days of d's weekday before it and two after, inside the
calendar, in date order.
-}
nearestFree : Calendar -> Day -> List Day
nearestFree cal d =
    let
        walk step x found =
            if List.length found == 2 || x < cal.today || x > cal.last then
                found

            else if Set.member x cal.taken then
                walk step (x + step) found

            else
                walk step (x + step) (found ++ [ x ])
    in
    List.reverse (walk -7 (d - 7) []) ++ walk 7 (d + 7) []


monthNames : List String
monthNames =
    [ "januar", "februar", "mars", "april", "mai", "juni", "juli", "august", "september", "oktober", "november", "desember" ]


weekdayNames : List String
weekdayNames =
    [ "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag", "søndag" ]


at : Int -> List String -> String
at i list =
    List.drop i list |> List.head |> Maybe.withDefault ""


capital : String -> String
capital s =
    String.toUpper (String.left 1 s) ++ String.dropLeft 1 s


monthName : Day -> String
monthName d =
    at (Day.month d - 1) monthNames


{-| "Lørdag 12. juni 2027"
-}
longDate : Day -> String
longDate d =
    capital (at (Day.weekday d) weekdayNames)
        ++ " "
        ++ String.fromInt (Day.day d)
        ++ ". "
        ++ monthName d
        ++ " "
        ++ String.fromInt (Day.year d)


{-| "lørdager"
-}
weekdayPlural : Day -> String
weekdayPlural d =
    at (Day.weekday d) weekdayNames ++ "er"


{-| "November 2026", "April – juni 2027", "November 2026 – januar 2027"
-}
monthRange : Day -> Day -> String
monthRange from to =
    let
        y =
            Day.year >> String.fromInt
    in
    if Day.monthsBetween from to == 0 then
        capital (monthName from) ++ " " ++ y from

    else if Day.year from == Day.year to then
        capital (monthName from) ++ " – " ++ monthName to ++ " " ++ y to

    else
        capital (monthName from) ++ " " ++ y from ++ " – " ++ monthName to ++ " " ++ y to


{-| How far the calendar goes, under the months.
-}
edge : Calendar -> String
edge cal =
    "Kalenderen går til " ++ monthName cal.last ++ " " ++ String.fromInt (Day.year cal.last) ++ ". Spør oss om datoer etter det."
