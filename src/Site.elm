module Site exposing (config)

import DataSource
import Head
import LanguageTag exposing (LanguageTag, emptySubtags)
import LanguageTag.Country as Country
import LanguageTag.Language
import MimeType
import Pages.Manifest as Manifest
import Pages.Url
import Path
import Route
import SiteConfig exposing (SiteConfig)


type alias Data =
    ()


config : SiteConfig Data
config =
    { data = data
    , canonicalUrl = "https://beta.aspargesgaarden.no"
    , manifest = manifest
    , head = head
    }


data : DataSource.DataSource Data
data =
    DataSource.succeed ()


head : Data -> List Head.Tag
head static =
    [ Head.sitemapLink "/sitemap.xml"
    ]


manifest : Data -> Manifest.Config
manifest static =
    Manifest.init
        { name = "Aspargesgården selskapslokaler"
        , description = "Sjarmerende, rustikk 1900-talls gård ved kysten i Vestfold"
        , startUrl = Route.Index |> Route.toPath
        , icons =
            [ { src = [ "ressurser", "favicon.svg" ] |> Path.join |> Pages.Url.fromPath
              , sizes =
                    [ ( 72, 72 )
                    , ( 96, 96 )
                    , ( 128, 128 )
                    , ( 256, 256 )
                    , ( 512, 512 )
                    , ( 1024, 1024 )
                    ]
              , mimeType = Just (MimeType.OtherImage "svg+xml")
              , purposes = []
              }
            ]
        }
        |> Manifest.withLang
            (LanguageTag.Language.no
                |> LanguageTag.build
                    { emptySubtags
                        | region = Just Country.no
                    }
            )
        |> Manifest.withShortName "Aspargesgården"
