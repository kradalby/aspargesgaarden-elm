module Day exposing
    ( Day
    , add
    , addMonths
    , day
    , fromIso
    , fromYmd
    , month
    , monthStart
    , monthsBetween
    , toIso
    , weekday
    , year
    )

{-| Calendar days with no time zone, as whole numbers: days since
1970-01-01. The server says which day it is in Oslo; nothing here reads a
clock. The conversions are Howard Hinnant's civil-date algorithms.
-}


type alias Day =
    Int


fromYmd : Int -> Int -> Int -> Day
fromYmd y m d =
    let
        y2 =
            if m <= 2 then
                y - 1

            else
                y

        era =
            y2 // 400

        yoe =
            y2 - era * 400

        doy =
            (153 * modBy 12 (m + 9) + 2) // 5 + d - 1

        doe =
            yoe * 365 + yoe // 4 - yoe // 100 + doy
    in
    era * 146097 + doe - 719468


toYmd : Day -> ( Int, ( Int, Int ) )
toYmd n =
    let
        z =
            n + 719468

        era =
            z // 146097

        doe =
            z - era * 146097

        yoe =
            (doe - doe // 1460 + doe // 36524 - doe // 146096) // 365

        doy =
            doe - (365 * yoe + yoe // 4 - yoe // 100)

        mp =
            (5 * doy + 2) // 153

        m =
            if mp < 10 then
                mp + 3

            else
                mp - 9

        y =
            yoe
                + era
                * 400
                + (if m <= 2 then
                    1

                   else
                    0
                  )
    in
    ( y, ( m, doy - (153 * mp + 2) // 5 + 1 ) )


year : Day -> Int
year =
    toYmd >> Tuple.first


month : Day -> Int
month =
    toYmd >> Tuple.second >> Tuple.first


day : Day -> Int
day =
    toYmd >> Tuple.second >> Tuple.second


{-| Monday 0 to Sunday 6. 1970-01-01 was a Thursday.
-}
weekday : Day -> Int
weekday n =
    modBy 7 (n + 3)


add : Int -> Day -> Day
add n d =
    d + n


monthStart : Day -> Day
monthStart d =
    fromYmd (year d) (month d) 1


{-| The 1st of the month n months after d's.
-}
addMonths : Int -> Day -> Day
addMonths n d =
    let
        i =
            year d * 12 + month d - 1 + n
    in
    fromYmd (i // 12) (modBy 12 i + 1) 1


monthsBetween : Day -> Day -> Int
monthsBetween from to =
    (year to - year from) * 12 + month to - month from


{-| A strict YYYY-MM-DD, as the date field and the server write it.
-}
fromIso : String -> Maybe Day
fromIso s =
    case String.split "-" s of
        [ y, m, d ] ->
            case ( String.toInt y, String.toInt m, String.toInt d ) of
                ( Just yy, Just mm, Just dd ) ->
                    let
                        n =
                            fromYmd yy mm dd
                    in
                    if String.length y == 4 && String.length m == 2 && String.length d == 2 && yy > 0 && toIso n == s then
                        Just n

                    else
                        Nothing

                _ ->
                    Nothing

        _ ->
            Nothing


toIso : Day -> String
toIso n =
    let
        ( y, ( m, d ) ) =
            toYmd n

        pad i =
            String.padLeft 2 '0' (String.fromInt i)
    in
    String.padLeft 4 '0' (String.fromInt y) ++ "-" ++ pad m ++ "-" ++ pad d
