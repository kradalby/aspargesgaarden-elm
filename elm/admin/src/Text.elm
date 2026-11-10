module Text exposing
    ( askForOriginal
    , atFloor
    , cropTooSmall
    , error
    , movedTo
    , noAccess
    , noCrop
    , photoFailure
    , problem
    , sharp
    , shownSmaller
    , softCrop
    , softKind
    , softWhole
    , uploadFailure
    , withoutScheme
    )

{-| Every message the user reads about something that went wrong, or about
how sharp a photo will be, in plain bokmål. The server sends codes and
numbers only (docs/admin-api.md), so this is the one place the wording
lives.
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
            cropTooSmall

        ( "crop_outside", _ ) ->
            "Rammen går utenfor bildet. Dra den litt innover."

        ( "crop_aspect", _ ) ->
            "Rammen har feil form. Trykk «Avbryt» og prøv igjen."

        ( "conflict", _ ) ->
            "Noen andre har lagret endringer her mens du jobbet."

        ( "forbidden", _ ) ->
            noAccess

        ( "wrong_host", _ ) ->
            "Administrasjonen må åpnes fra sin egen adresse. Bruk lenken du har fått, ikke en kopi av siden."

        ( "cross_origin", _ ) ->
            "Endringen kom ikke fra administrasjonen selv, så den ble stoppet. Last inn siden på nytt og prøv igjen."

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

        "too_many_pixels" ->
            "Bildet har for mange punkter til at nettsiden kan behandle det (over 100 millioner). Lagre det i mindre størrelse, og prøv igjen."

        _ ->
            "Noe gikk galt på serveren. Prøv igjen om litt."


{-| How sharp a photo will be on the site. Below a kind's sharp width the
browser stretches it on every screen; the server takes it all the same.
-}
sharp : String
sharp =
    "Skarpt."


softCrop : String
softCrop =
    "Dette utsnittet blir uskarpt på nettsiden. Gjør rammen større, eller bruk et større bilde om du har det."


cropTooSmall : String
cropTooSmall =
    "Utsnittet er for lite. Gjør rammen større."


{-| Why "Mindre" stops: the server's minimum for the kind.
-}
atFloor : String
atFloor =
    "Rammen kan ikke bli mindre, for nettsiden trenger minst så mye av bildet."


{-| Beside the photo in the form, for a kind whose crop is soft.
-}
softKind : String -> String
softKind usedFor =
    usedFor ++ ": utsnittet blir uskarpt på nettsiden. Trykk «Beskjær» og gjør rammen større, eller bruk et større bilde om du har det."


{-| What a photo too small for a crop kind gets instead. minWidth is the
server's minimum for the kind.
-}
noCrop : String -> Int -> String
noCrop key minWidth =
    let
        under =
            "under " ++ String.fromInt minWidth ++ " punkter bredt"
    in
    case key of
        "share" ->
            "Bildet er " ++ under ++ ", så det får ikke eget bilde for deling på Facebook. Da brukes bildet fra toppen av siden. Det går fint."

        "banner" ->
            "Bildet er for lite til toppen av siden (" ++ under ++ "), så der vises det uten beskjæring."

        _ ->
            "Bildet er for lite til listen (" ++ under ++ "), så der vises det uten beskjæring."


{-| Follows noCrop when the whole photo shown there is soft.
-}
softWhole : String
softWhole =
    "Der blir det uskarpt. Bruk et større bilde om du har det."


{-| With every warning about softness: the usual cause, and the cure.
-}
askForOriginal : String
askForOriginal =
    "Bilder sendt på Messenger eller e-post blir ofte gjort mindre. Be om originalen."


{-| A gallery or extra photo narrower than the lightbox: it is not
stretched, only shown smaller, so this is information, not a warning.
-}
shownSmaller : String
shownSmaller =
    "Bildet er litt lite, så det vises mindre når man åpner det stort. Det går fint."


{-| Said aloud after a move button, with the item's new place.
-}
movedTo : Int -> String
movedTo place =
    "Bildet er flyttet til plass " ++ String.fromInt place ++ "."


{-| For someone the tailnet does not let into the admin.
-}
noAccess : String
noAccess =
    "Du har ikke tilgang til administrasjonen. Be Kristoffer om å gi deg tilgang."


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


{-| An address as she would type it: without <https://>.
-}
withoutScheme : String -> String
withoutScheme url =
    if String.startsWith "https://" url then
        String.dropLeft 8 url

    else if String.startsWith "http://" url then
        String.dropLeft 7 url

    else
        url
