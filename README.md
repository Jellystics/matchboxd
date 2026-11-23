# matchboxd

Petite application web pour tirer au sort **un film à regarder à deux**, à partir de deux comptes [Letterboxd](https://letterboxd.com).

1. Entrez deux pseudos Letterboxd.
2. Pour chaque personne, choisissez une source : watchlist, films vus, films aimés ou une liste personnalisée.
3. Cliquez sur **Tirer un film** : l’application affiche un film présent dans les **deux** sources (affiche, réalisateur, durée, genres, synopsis, note Letterboxd, note IMDb si une clé OMDb est configurée).
4. **Un autre** retire un film parmi les films en commun, sans répéter celui affiché.

L’interface est disponible en **français et en anglais** (bouton FR | EN en haut à droite ; langue du navigateur par défaut, choix mémorisé). Les synopsis restent dans la langue fournie par Letterboxd (anglais).

Tout tient dans un seul exécutable Go : l’API JSON et l’interface (HTML/CSS/JS embarqués avec `go:embed`).

## Compiler et lancer

Prérequis : Go ≥ 1.27 (ou `nix develop` grâce au `flake.nix` du dépôt).

```sh
go build -o app .
./app
```

Puis ouvrez <http://localhost:8080>.

Avec [Task](https://taskfile.dev) (fourni par le flake), le binaire embarque aussi sa version (tirée de `git describe`) :

```sh
task build   # binaire statique ./app versionné
task run     # compile et lance
./app -version
```

Binaires précompilés (Linux, macOS, Windows, FreeBSD ; amd64, arm64, arm…) : voir les [Releases GitHub](https://github.com/Jellystics/matchboxd/releases).

## Docker

Image multi-architecture publiée sur GHCR : `linux/amd64`, `arm64`, `arm/v7`, `arm/v6`, `386`, `ppc64le`, `s390x`, `riscv64`.

```sh
docker run -d --name matchboxd -p 8080:8080 \
  -e OMDB_API_KEY=abcd1234 \
  -v matchboxd-data:/data \
  ghcr.io/jellystics/matchboxd:latest
```

- L’image part de `scratch` (~19 Mo) et tourne en utilisateur non-root (`65532`).
- Le volume `/data` conserve la session Letterboxd (`/data/session.json`) entre les redémarrages.
- Tags disponibles : `latest` et `1.2.3` / `1.2` / `1` pour les versions, `edge` pour la branche `main`, `sha-abc1234` pour chaque commit.

Pour construire l’image localement : `task docker` (ou `docker build -t matchboxd .`).

## Versions et publication

Le projet suit le [versionnage sémantique](https://semver.org/lang/fr/) via des tags git `vX.Y.Z`. Le workflow GitHub Actions (`.github/workflows/ci.yml`) :

| Événement            | Tests | Image Docker                             | Release GitHub |
|----------------------|-------|------------------------------------------|----------------|
| Pull request         | ✅    | construite, non publiée                  | —              |
| Push sur `main`      | ✅    | publiée en `edge` et `sha-…`             | —              |
| Tag `v1.2.3`         | ✅    | publiée en `1.2.3`, `1.2`, `1`, `latest` | binaires + sommes SHA-256 |

Pour publier une version :

```sh
task release -- v1.2.3     # crée le tag annoté et le pousse ; la CI fait le reste
```

Un tag avec suffixe (`v1.2.3-rc.1`) produit une pré-release. Les versions `v0.x` ne reçoivent pas le tag Docker majeur `0`.

### Configuration

| Variable d’environnement | Option          | Défaut | Rôle                                          |
|--------------------------|-----------------|--------|-----------------------------------------------|
| `PORT`                   | `-port`         | `8080` | Port HTTP                                     |
| `OMDB_API_KEY`           | `-omdb-key`     | —      | Clé OMDb pour afficher la note IMDb (optionnel) |
| —                        | `-cache-ttl`    | `10m`  | Durée de conservation des pages en mémoire    |
| `MATCHBOXD_SESSION`      | `-session`      | cache utilisateur | Fichier où la session Letterboxd (cookies) est conservée |
| `MATCHBOXD_BROWSER`      | `-browser`      | `chrome_152` | Version de Chrome dont l’empreinte TLS est imitée |

Le fichier de session vaut par défaut `~/Library/Caches/matchboxd/session.json` (macOS),
`~/.cache/matchboxd/session.json` (Linux) ou `%LocalAppData%\matchboxd\session.json` (Windows).
Supprimez-le pour repartir d’une session neuve.

```sh
OMDB_API_KEY=abcd1234 PORT=3000 ./app
```

Sans clé OMDb, la note IMDb est simplement masquée (le lien IMDb reste affiché).

## Obtenir une clé OMDb gratuite

1. Allez sur <https://www.omdbapi.com/apikey.aspx>.
2. Choisissez l’offre **FREE** (1 000 requêtes par jour), renseignez votre e-mail et un usage (« personal project » suffit).
3. Cliquez sur le lien d’activation reçu par e-mail.
4. Lancez l’application avec `OMDB_API_KEY=<votre clé> ./app`.

## Tests

```sh
go test ./...
```

Les tests unitaires du parsing HTML utilisent de vraies pages Letterboxd enregistrées dans `internal/letterboxd/testdata/`.

Un test « en direct » interroge le vrai site (comptes publics `davidehrlich` et `karsten`) pour vérifier que le parsing fonctionne toujours. Il est désactivé par défaut :

```sh
MATCHBOXD_LIVE=1 go test ./internal/letterboxd -run Live -v
```

Si Letterboxd modifie son HTML, enregistrez de nouvelles pages dans `testdata/` et ajustez `parse.go`.

## Fonctionnement

- Letterboxd n’a pas d’API publique : le serveur Go lit les pages publiques (`/<user>/watchlist/`, `/<user>/films/`, `/<user>/likes/films/`, `/<user>/lists/`, `/<user>/list/<liste>/`, `/film/<film>/`) et les analyse avec [goquery](https://github.com/PuerkitoBio/goquery).
- Les requêtes imitent un onglet Chrome : empreinte TLS et HTTP/2 de Chrome 152 ([tls-client](https://github.com/bogdanfinn/tls-client)), en-têtes dans l’ordre du navigateur (`sec-ch-ua`, `Sec-Fetch-*`, `Referer`, `Accept-Language`…).
- Chaque lancement reprend la même session : une navigation de préchauffage ouvre l’accueil, puis les cookies Cloudflare (`__cf_bm`, `cf_clearance`) sont conservés dans le fichier de session et rechargés au démarrage suivant.
- Toutes les pages d’une source sont récupérées (quelques requêtes en parallèle, espacées aléatoirement de 120 à 320 ms).
- Les films sont comparés par leur *slug* Letterboxd, pas par leur titre.
- Les pages analysées sont gardées en mémoire quelques minutes : les tirages suivants sont instantanés.

### Limites connues

Letterboxd est protégé par Cloudflare. L’empreinte TLS de Chrome et la session persistante suffisent pour la plupart des pages (watchlists, films vus, films aimés, listes et fiches de films passent en pratique), mais Cloudflare peut encore présenter un **défi JavaScript** selon le réseau ou l’IP. Ce défi ne peut être résolu qu’en exécutant du JavaScript dans un vrai navigateur : l’application ne le fait pas et affiche alors un message clair (« Letterboxd bloque temporairement nos requêtes… »). Pour l’index des listes, elle propose les listes de la première page avec un avertissement.

## Structure

```
main.go                      point d’entrée : configuration, serveur HTTP
internal/letterboxd/         client Letterboxd : requêtes, cache, parsing HTML
  testdata/                  pages Letterboxd enregistrées pour les tests
internal/omdb/               note IMDb via l’API OMDb
internal/server/             API JSON (/api/sources, /api/pick) et fichiers statiques
web/                         interface : index.html, style.css, app.js (embarqués)
Dockerfile                   image multi-architecture (cross-compilation Go, base scratch)
Taskfile.yml                 build versionné, tests, image Docker, publication
.github/workflows/ci.yml     tests, images GHCR, releases
```

### API

- `GET /api/sources?user=<pseudo>` → sources disponibles pour un compte.
- `POST /api/pick` avec `{"a":{"user":"…","source":"watchlist"},"b":{"user":"…","source":"list:<slug>"},"exclude":"<slug>"}` → `{"count": 126, "film": {…}, "imdb": {…}, "sizes": [1196, 632]}`.
- En cas d’erreur : `{"code": "user_not_found", "user": "…", "error": "…"}`. L’interface traduit le message à partir de `code` (`user_not_found`, `list_not_found`, `private`, `blocked`, `timeout`, `upstream`, …).
