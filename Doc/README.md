# Go Markdown Server — Architecture Documentation

Date: 2025-11-26

## Overview
This project serves Markdown documentation from a MongoDB-backed store, renders PlantUML diagrams via an external PlantUML server, and provides a simple web UI with search, collections, and smooth anchor navigation.

- Backend: Go 1.21 + Gorilla Mux + Blackfriday
- Database: MongoDB (`blog.posts`)
- Diagrams: `plantuml/plantuml-server:jetty` (Docker)
- Frontend: Single-page template `md.html` with vanilla JavaScript
- Orchestration: `docker-compose.yml`

## Components
- `main.go`: Server boot, router setup, env config (`PORT`, `SYNC_DIR`)
- `routes.go`: HTTP handlers (index, post, collections, collection, API endpoints)
- `db/datebase.go`: MongoDB access layer (CRUD, queries)
- `plantuml/plantuml.go`: PlantUML processing for code blocks and `.puml` files
- `md.html`: HTML/CSS/JS template (search, navigation, rendering)

## Data Model
Collection: `blog.posts`

```json
{
  "title": "string",
  "body": "string",       // Markdown processed with PlantUML + link normalization
  "url": "string",        // slug used in routes like /post/{url}
  "collection": "string", // logical grouping (e.g., Architecture)
  "isIndex": false         // true for collection index pages
}
```

## Routing & Endpoints
- `GET /`: Home. If an index post exists (global), renders it; otherwise lists all posts.
- `GET /post/{name}`: Render a single post by `url`.
- `GET /collections`: JSON array of all collection names.
- `GET /collection/{collection}`: Renders collection index or a list of posts.
- `GET /api/collection/{name}`: JSON array of posts for a collection.
- `POST /api/collection/create`: Create a collection; optionally upload markdown files.
- `DELETE /api/collection/{name}/delete`: Delete a collection and its posts.
- `POST /api/collection/{name}/upload`: Upload markdown files to an existing collection.
- `POST|GET /api/sync`: Sync files from `SYNC_DIR` into MongoDB (via `filesync`).

## Frontend Behavior (md.html)
- Search bar with debounce (300ms):
  - Loads all collections and their posts on startup via `/collections` + `/api/collection/{name}`.
  - Filters by query (min length 2) and selected collection.
  - Results show title, collection, contextual snippet; clicking opens in a new tab.
- Anchor link smoothing: Intra-page heading navigation with fuzzy match.
- Result panel stays open until Escape or clicking outside.

## PlantUML Integration
- Code fences: ```plantuml ... ``` are detected and transformed into diagram images.
- `.puml` files: Image references `![title](path/to/file.puml)` are loaded, optionally injected with skinparams, encoded, and rendered.
- Server URLs:
  - Internal: `PLANTUML_SERVER` (defaults to `http://plantuml:8080`) for validation/fetching.
  - Public: `PLANTUML_PUBLIC_URL` (defaults to `http://localhost:8081`) used in image `src` for the browser.
- Font sizing via `skinparam` for readability alignment with text.

### Services Architecture Diagram
The following diagram shows the relationship between the browser, Go service, MongoDB and the PlantUML server:

![Services Architecture](Doc/diagrams/services-architecture.puml)

Config note: The PlantUML server endpoints are provided via environment variables:
- `PLANTUML_SERVER` (internal service URL)
- `PLANTUML_PUBLIC_URL` (browser-facing URL)
These are set in `docker-compose.yml` and can be overridden per environment without code changes.

## Link Normalization
`processCrossReferences` adjusts relative links:
- `[text](./file.md)` → `[text](/post/file)`
- `[text](file.md)` → `[text](/post/file)`
- Strips `.md` extensions in post links.

## Environment Variables
- `PORT`: Web server port (default `8080`).
- `SYNC_DIR`: Directory for file sync when using `/api/sync` (default `./content`).
- `PLANTUML_SERVER`: Internal PlantUML server URL (default `http://plantuml:8080`).
- `PLANTUML_PUBLIC_URL`: Public URL for browser to fetch images (default `http://localhost:8081`).

## Docker Compose
Services:
- `web` (go-markdown-server)
- `mongo`
- `plantuml`

Ports:
- Web: `8080:8080`
- Mongo: `27017:27017`
- PlantUML: `8081:8080` (external 8081 → container 8080)

Volumes:
- Mounts `md.html` read-only into the container to allow quick template changes.

## Typical Workflow
1. Start services:
   ```bash
   docker compose up -d --build
   ```
2. Upload documentation via UI or API (`/api/collection/create` or `/api/collection/{name}/upload`).
3. Browse collections at `http://localhost:8080/collection/{CollectionName}`.
4. Use search from the top navigation to find content across collections.

## API Examples
- List collections:
  ```bash
  curl http://localhost:8080/collections
  ```
- Posts in collection:
  ```bash
  curl http://localhost:8080/api/collection/Architecture
  ```
- Create empty collection:
  ```bash
  curl -X POST -F name=Docs http://localhost:8080/api/collection/create
  ```
- Upload files to collection:
  ```bash
  curl -X POST -F files=@Architecture/index.md -F files=@Architecture/build-system.md \
       http://localhost:8080/api/collection/Architecture/upload
  ```

## Error Handling
- 404: Renders a Markdown-based error page via `errorNotFoundPage`.
- 500: Renders a Markdown-based error page via `internalServerErrorPage` and logs the panic.

## Future Enhancements
- Server-side full-text search index for scalability.
- Pagination for large collections.
- PDF export pipelines integrated for each collection.
- Authentication for write endpoints.

## Maintenance Notes
- Keep `md.html` consistent with API field names (lowercase: `title`, `body`, `url`).
- Ensure PlantUML server is reachable; diagrams use `PLANTUML_PUBLIC_URL` in rendered pages.
- Validate MongoDB connectivity before enabling sync operations.
