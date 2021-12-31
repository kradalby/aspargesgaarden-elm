module Data.Photo exposing (Gallery, Photo, Photographer, photoDecoder, photographerDecoder, photographersDecoder, yamlFileToDataSourceGallery, yamlToPhotographers)

import DataSource exposing (DataSource)
import DataSource.File as File
import Dict
import Route exposing (Route(..))
import Yaml.Decode as Y


type alias Photographer =
    { name : String
    , website : String
    }


type alias PhotoYAML =
    { path : String
    , photographer : String
    , description : String
    }


type alias Photo =
    { path : String
    , photographer : Photographer
    , description : String
    }


type alias GalleryYAML =
    { photos : List PhotoYAML
    }


type alias Gallery =
    { photos : List Photo
    }


photographersDecoder : Y.Decoder (Dict.Dict String Photographer)
photographersDecoder =
    Y.field "photographers" (Y.dict photographerDecoder)


photographerDecoder : Y.Decoder Photographer
photographerDecoder =
    Y.map2 Photographer
        (Y.field "name" Y.string)
        (Y.field "website" Y.string)


photoDecoder : Y.Decoder PhotoYAML
photoDecoder =
    Y.map3 PhotoYAML
        (Y.field "path" Y.string)
        (Y.field "photographer" Y.string)
        (Y.field "description" Y.string)


yamlToPhoto : Dict.Dict String Photographer -> PhotoYAML -> Photo
yamlToPhoto photographers photo =
    { path = photo.path
    , photographer = Dict.get photo.photographer photographers |> Maybe.withDefault { name = "Ukjent", website = "" }
    , description = photo.description
    }


yamlToPhotographers : String -> Dict.Dict String Photographer
yamlToPhotographers content =
    Y.fromString
        photographersDecoder
        content
        |> Result.withDefault Dict.empty


yamlFileToDataSourceGallery : String -> DataSource Gallery
yamlFileToDataSourceGallery path =
    -- TODO: There should be some error handling here
    File.rawFile
        path
        |> DataSource.map yamlToPhotographers
        |> DataSource.andThen
            (\photographers ->
                File.rawFile
                    path
                    |> DataSource.map
                        (\content ->
                            Y.fromString
                                (Y.map GalleryYAML
                                    (Y.field "photos" (Y.list photoDecoder))
                                )
                                content
                                |> Result.withDefault { photos = [] }
                        )
                    |> DataSource.map
                        (\gallery ->
                            let
                                photos =
                                    List.map (yamlToPhoto photographers) gallery.photos
                            in
                            { photos = photos }
                        )
            )
