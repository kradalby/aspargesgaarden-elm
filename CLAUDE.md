# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an Elm/elm-pages 2 static site generator project for aspargesgaarden.no, built with Nix flakes. The site features events, facilities, galleries, and contact information.

## Development Commands

### Using Nix (recommended)
```bash
# Enter development environment with all tools
nix develop

# Build the site
elm-pages build

# Development server with hot reload
elm-pages dev

# Format Elm code
elm-format src/ --yes

# Generate Tailwind CSS Elm modules
yarn elm-tailwind-modules --tailwind-config tailwind.config.js --dir ./src
```

### Using Yarn (requires Node.js)
```bash
# Install dependencies
yarn install

# Development server
yarn start

# Build production site
yarn build
```

### Image Processing
```bash
# Resize and optimize images (requires imagemagick and optimizt)
./resize_images.sh
```

## Architecture

### Core Structure
- **elm-pages 2**: Static site generator with file-based routing
- **Nix Flakes**: Reproducible build environment and deployment
- **Tailwind CSS**: Utility-first CSS with Elm type-safe bindings via elm-tailwind-modules

### Key Files
- `src/Site.elm`: Site configuration, SEO settings, and manifest
- `src/Shared.elm`: Shared data and types across pages
- `src/View.elm`: Main layout and page wrapper
- `src/Page/*.elm`: Individual page modules (Index, About, Events, Gallery, etc.)
- `plugins/MarkdownCodec.elm`: Custom markdown processing for content files

### Content Management
- `content/events/*.md`: Event markdown files with frontmatter (date, title, image, description)
- `content/facilities/*.md`: Facility descriptions
- `content/gallery.yaml`: Gallery image configuration
- `public/`: Static assets including optimized images in multiple formats (JPEG, WebP, AVIF)

### Build System
The Nix flake (`flake.nix`) provides:
- Elm 0.19.1 environment
- Node.js and Yarn for dependencies
- elm-pages CLI and build tools
- Image processing utilities (imagemagick, optimizt)

The build process:
1. Fetches Elm dependencies via `elm-srcs.nix` and `registry.dat`
2. Links node_modules from Yarn packages
3. Runs `elm-pages build` to generate static site
4. Outputs to `dist/` directory

## Key Patterns

### Page Development
Pages in `src/Page/` follow the elm-pages 2 pattern:
- `type alias Data`: Page-specific data from DataSource
- `data`: DataSource for fetching content (markdown, YAML, etc.)
- `head`: SEO and meta tags
- `view`: Page rendering using elm-css and Tailwind utilities

### Responsive Images
The site uses responsive image optimization:
- Multiple sizes (320w to 2048w) for different viewport widths
- Modern formats (WebP, AVIF) with JPEG fallbacks
- Generated via `resize_images.sh` script

### Styling
- Tailwind utilities generated as Elm modules in `src/Tailwind/`
- Custom markdown renderer in `TailwindMarkdownRenderer.elm`
- Responsive breakpoints handled via `Tailwind.Breakpoints`