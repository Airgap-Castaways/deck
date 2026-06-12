# Server Registry

`deck server up` exposes a read-only OCI Distribution v2 registry at the `/v2` prefix. It serves the container image archives that `deck prepare` downloaded into the bundle, so that runtimes such as `containerd`, `docker`, or `crane` can pull images directly from the local server without any external network access.

## What it is

The registry is pull-only. It does not accept pushes or any write operations. Every request that is not `GET` or `HEAD` returns `405 Method Not Allowed`.

The registry discovers images by scanning `outputs/images/` (and the legacy `images/` path) under the bundle root for `.tar` files. Each tar is read using the Docker tarball format. Files excluded by `.deckignore` are skipped.

See [Bundle Layout](../bundle-layout.md) for how `outputs/images/` is populated.

## Endpoints

`GET` and `HEAD` responses set the `Docker-Distribution-API-Version: registry/2.0` response header. Rejected non-`GET`/`HEAD` requests receive `405 Method Not Allowed` without this header.

### Ping

```
GET  /v2
HEAD /v2
GET  /v2/
HEAD /v2/
```

Returns `200 OK` with an empty body. Standard OCI/Docker registry discovery check.

### Catalog

```
GET  /v2/_catalog
HEAD /v2/_catalog
```

Returns a JSON object listing all repository names the server can serve.

```json
{"repositories": ["calico/node-driver-registrar", "quay.io/calico/node-driver-registrar"]}
```

### Tags list

```
GET  /v2/<name>/tags/list
HEAD /v2/<name>/tags/list
```

Returns a JSON object listing all tags for the named repository. Returns `404` if the repository is not found.

```json
{"name": "calico/node-driver-registrar", "tags": ["v3.28.0"]}
```

### Manifest

```
GET  /v2/<name>/manifests/<ref>
HEAD /v2/<name>/manifests/<ref>
```

`<ref>` may be a tag (e.g. `3.18`) or a digest (e.g. `sha256:abc123…`). Returns the raw image manifest with the appropriate `Content-Type` and a `Docker-Content-Digest` header. Returns `404` if the image is not found.

### Blob

```
GET  /v2/<name>/blobs/<digest>
HEAD /v2/<name>/blobs/<digest>
```

Returns the image config or a compressed layer identified by `<digest>`. Returns `404` if the digest is not found within the named repository.

## Repository name aliases

Image tarballs embed a `RepoTags` field. The server parses each tag with weak validation and derives the canonical repository name. For example, a tag of `quay.io/calico/node-driver-registrar:v3.28.0` has canonical repository `quay.io/calico/node-driver-registrar`.

When the first path component of the canonical name looks like a registry domain (contains `.` or `:`, or equals `localhost`), the server registers a second alias that strips the domain prefix. Using the example above, both of the following names refer to the same image and are interchangeable:

- `quay.io/calico/node-driver-registrar` (canonical, domain-prefixed)
- `calico/node-driver-registrar` (alias, domain stripped)

This lets clients pull using either the fully-qualified name or the shorter form.

> **Docker Hub note:** go-containerregistry normalizes `docker.io` to `index.docker.io`, so a tag stored as `docker.io/library/alpine:3.18` is registered under the canonical name `index.docker.io/library/alpine`. Clients should pull via `index.docker.io/library/alpine` or the domain-stripped alias `library/alpine`.

## Read-only enforcement

Any method other than `GET` or `HEAD` — including `POST`, `PUT`, `DELETE`, and `PATCH` — returns `405 Method Not Allowed` before any path dispatch occurs. There are no push, delete, or upload endpoints.

## `.deckignore`

The server loads `.deckignore` from the bundle root before scanning for image tarballs. Any `.tar` file whose path (relative to the bundle root) matches an ignore rule is excluded from the catalog and cannot be served.

## Usage example

Start the server with the default address:

```sh
deck server up --root /path/to/bundle
```

The registry listens on the same address as the rest of the server (default `:8080`, configurable with `--addr`). Pull an image using `crane`:

```sh
crane pull localhost:8080/calico/node-driver-registrar:v3.28.0 node-driver-registrar.tar
```

Or configure `containerd` to use the server as a mirror for a registry, pointing it at `http://localhost:8080`.

For TLS options and daemon mode see [deck server up](../cli/deck_server_up.md).
