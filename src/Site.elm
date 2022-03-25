module Site exposing (commonSeo, config)

import DataSource
import Head
import Head.Seo as Seo
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


iconSizes : List ( Int, Int )
iconSizes =
    [ ( 16, 16 )
    , ( 24, 24 )
    , ( 32, 32 )
    , ( 48, 48 )
    , ( 64, 64 )
    , ( 72, 72 )
    , ( 80, 80 )
    , ( 96, 96 )
    , ( 128, 128 )
    , ( 256, 256 )
    ]


iconSvgSizes : List ( Int, Int )
iconSvgSizes =
    iconSizes
        ++ [ ( 512, 512 )
           , ( 1024, 1024 )
           ]


iconSvgUrl : Pages.Url.Url
iconSvgUrl =
    [ "ressurser", "favicon.svg" ] |> Path.join |> Pages.Url.fromPath


iconUrl : Pages.Url.Url
iconUrl =
    [ "ressurser", "favicon.ico" ] |> Path.join |> Pages.Url.fromPath


data : DataSource.DataSource Data
data =
    DataSource.succeed ()


head : Data -> List Head.Tag
head static =
    [ Head.sitemapLink "/sitemap.xml"
    , Head.icon iconSvgSizes (MimeType.OtherImage "svg+xml") iconSvgUrl
    , Head.icon iconSizes (MimeType.OtherImage "x-icon") iconUrl
    , Head.appleTouchIcon (Just 180) iconSvgUrl
    , Head.appleTouchIcon (Just 192) iconSvgUrl
    ]


manifest : Data -> Manifest.Config
manifest static =
    Manifest.init
        { name = "Aspargesgården selskapslokaler"
        , description = "Sjarmerende, rustikk 1900-talls gård ved kysten i Vestfold"
        , startUrl = Route.Index |> Route.toPath
        , icons =
            [ { src = iconSvgUrl
              , sizes =
                    iconSvgSizes
              , mimeType = Just (MimeType.OtherImage "svg+xml")
              , purposes = []
              }
            , { src = iconUrl
              , sizes =
                    iconSizes
              , mimeType = Just (MimeType.OtherImage "x-icon")
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


commonSeo :
    { canonicalUrlOverride : Maybe String
    , siteName : String
    , image : Seo.Image
    , description : String
    , title : String
    , locale : Maybe String
    }
commonSeo =
    { canonicalUrlOverride = Nothing
    , siteName = "Aspargesgården"
    , image =
        { url = [ "ressurser", "twitter_2048w_resize.jpeg" ] |> Path.join |> Pages.Url.fromPath
        , alt = "Aspargesgården"
        , dimensions = Just { width = 2048, height = 1024 }
        , mimeType = Just "image/jpeg"
        }
    , description = "Aspargesgården byr på sjarmerende og unike lokaler i landlige omgivelser, ved kysten i Vestfold."
    , locale = Just "nb_NO"
    , title = "Aspargesgården | "
    }
