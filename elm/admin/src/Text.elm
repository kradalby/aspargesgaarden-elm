module Text exposing (error, photoFailure, problem, uploadFailure)

{-| Every message the user reads about something that went wrong, in plain
bokmål. The server sends codes only (docs/admin-api.md), so this is the one
place the wording lives.
-}

import Api exposing (Problem)


problem : Problem -> String
problem p =
    case ( p.code, p.field ) of
        ( "title_required", _ ) ->
            "Skriv en tittel."

        ( "start_required", _ ) ->
            "Velg dagen arrangementet starter."

        ( "invalid_date", _ ) ->
            "Datoen er ikke gyldig. Velg den i kalenderen."

        ( "invalid_time", _ ) ->
            "Skriv klokkeslettet som timer og minutter, for eksempel 18:30."

        ( "slug_invalid", _ ) ->
            "Adressen kan bare ha små bokstaver uten æ, ø og å, tall og bindestrek, og kan ikke bare være tall."

        ( "slug_reserved", _ ) ->
            "Denne adressen brukes av en fast side på nettsiden. Velg en annen."

        ( "slug_taken", _ ) ->
            "Et annet arrangement samme år har allerede denne adressen. Endre den litt, for eksempel ved å legge til et ord."

        ( "slug_frozen", _ ) ->
            "Adressen kan ikke endres etter at arrangementet er publisert, så lenker som er delt fortsatt virker."

        ( "end_before_start", Just "endTime" ) ->
            "Arrangementet kan ikke slutte før det starter. Sjekk klokkeslettene."

        ( "end_before_start", _ ) ->
            "Siste dag kan ikke være før første dag."

        ( "extra_date", _ ) ->
            "Ekstra datoer må komme etter første dag, og hver dato bare én gang."

        ( "ticket_url", _ ) ->
            "Lenken må være en hel nettadresse som starter med https://. Kopier den fra nettleseren og lim den inn."

        ( "main_photo_required", _ ) ->
            "Velg et bilde. Det vises i listen, øverst på siden og når noen deler lenken."

        ( "alt_required", _ ) ->
            "Beskriv bildet med noen ord."

        ( "duplicate_photo", _ ) ->
            "Det samme bildet er brukt to ganger. Ta bort det ene."

        ( "not_found", _ ) ->
            "Fant det ikke. Kanskje noen andre har slettet det."

        ( "photographer_name_required", _ ) ->
            "Skriv navnet til fotografen."

        ( "photographer_url", _ ) ->
            "Lenken må være en hel nettadresse som starter med https://."

        ( "photographer_name_taken", _ ) ->
            "Det finnes allerede en fotograf med dette navnet."

        ( "crop_too_small", _ ) ->
            "Utsnittet er for lite. Gjør rammen større."

        ( "crop_outside", _ ) ->
            "Rammen går utenfor bildet. Dra den litt innover."

        ( "crop_aspect", _ ) ->
            "Rammen har feil form. Trykk «Avbryt» og prøv igjen."

        _ ->
            uploadFailure p.code


{-| Why an upload did not go through, and what to do about it.
-}
uploadFailure : String -> String
uploadFailure code =
    case code of
        "too_large" ->
            "Filen er for stor (over 40 MB). Velg en mindre utgave av bildet."

        "unsupported_type" ->
            "Dette er ikke et bilde nettsiden kan bruke. Velg et vanlig foto (JPEG, PNG eller HEIC)."

        "damaged" ->
            "Filen ser ut til å være ødelagt. Velg bildet på nytt, eller velg et annet."

        "too_small" ->
            "Bildet er for lite (under 480 punkter bredt). Velg et større bilde."

        _ ->
            "Noe gikk galt på serveren. Prøv igjen om litt."


{-| Why the server gave up preparing a photo's sizes.
-}
photoFailure : String -> String
photoFailure reason =
    case reason of
        "damaged" ->
            "Bildet kunne ikke leses. Filen er nok ødelagt. Ta det bort og velg et annet."

        "too_small" ->
            "Bildet er for lite til å brukes. Ta det bort og velg et større."

        _ ->
            "Bildet kunne ikke klargjøres. Trykk «Prøv igjen»."


{-| A whole request failing. Says plainly that nothing typed is lost, since
the form keeps everything.
-}
error : Api.Error -> String
error e =
    case e of
        Api.Network ->
            "Fikk ikke kontakt med serveren. Sjekk at du er på nett, og prøv igjen. Det du har skrevet er ikke borte."

        Api.Problems ps ->
            String.join " " (List.map problem ps)

        Api.Conflict _ ->
            "Noen andre har lagret endringer her mens du jobbet."

        Api.PhotoInUse _ ->
            "Bildet er i bruk."

        Api.NameTaken _ ->
            "Det finnes allerede en fotograf med dette navnet."

        Api.Unexpected ->
            "Noe gikk galt på serveren. Prøv igjen om litt. Det du har skrevet er ikke borte."
