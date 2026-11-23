# matchboxd

Petite application web pour tirer au sort **un film à regarder à deux**, à partir de deux comptes [Letterboxd](https://letterboxd.com).

1. Entrez deux pseudos Letterboxd.
2. Pour chaque personne, choisissez une source : watchlist, films vus, films aimés ou une liste personnalisée.
3. Cliquez sur **Tirer un film** : l’application affiche un film présent dans les **deux** sources (affiche, réalisateur, durée, genres, synopsis, note Letterboxd, note IMDb si une clé OMDb est configurée).
4. **Un autre** retire un film parmi les films en commun, sans répéter celui affiché.

Tout tient dans un seul exécutable Go : l’API JSON et l’interface (HTML/CSS/JS embarqués avec `go:embed`).

## Compiler et lancer

Prérequis : Go ≥ 1.26 (ou `nix develop` grâce au `flake.nix` du dépôt).

```sh
go build -o app .
./app
```

Puis ouvrez <http://localhost:8080>.

Pour un binaire statique (prêt pour un conteneur `scratch`/`distroless`) :

```sh
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o app .
```

### Configuration

| Variable d’environnement | Option          | Défaut | Rôle                                          |
|--------------------------|-----------------|--------|-----------------------------------------------|
| `PORT`                   | `-port`         | `8080` | Port HTTP                                     |
| `OMDB_API_KEY`           | `-omdb-key`     | —      | Clé OMDb pour afficher la note IMDb (optionnel) |
| —                        | `-cache-ttl`    | `10m`  | Durée de conservation des pages en mémoire    |

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
- Toutes les pages d’une source sont récupérées (quelques requêtes en parallèle, au maximum ~6 par seconde).
- Les films sont comparés par leur *slug* Letterboxd, pas par leur titre.
- Les pages analysées sont gardées en mémoire quelques minutes : les tirages suivants sont instantanés.

### Limites connues

Letterboxd est protégé par Cloudflare, qui peut présenter un défi anti-robots à certaines pages selon le réseau d’où viennent les requêtes. Lors du développement, depuis une connexion résidentielle, les **watchlists**, **listes personnalisées** et **fiches de films** passaient, alors que les **films aimés** et les pages 2+ des **films vus** et de l’index des listes étaient bloqués. L’application ne contourne pas ces protections : elle affiche un message clair (« Letterboxd bloque temporairement nos requêtes… ») et, pour l’index des listes, propose les listes de la première page avec un avertissement.

## Structure

```
main.go                      point d’entrée : configuration, serveur HTTP
internal/letterboxd/         client Letterboxd : requêtes, cache, parsing HTML
  testdata/                  pages Letterboxd enregistrées pour les tests
internal/omdb/               note IMDb via l’API OMDb
internal/server/             API JSON (/api/sources, /api/pick) et fichiers statiques
web/                         interface : index.html, style.css, app.js (embarqués)
```

### API

- `GET /api/sources?user=<pseudo>` → sources disponibles pour un compte.
- `POST /api/pick` avec `{"a":{"user":"…","source":"watchlist"},"b":{"user":"…","source":"list:<slug>"},"exclude":"<slug>"}` → `{"count": 126, "film": {…}, "imdb": {…}, "sizes": [1196, 632]}`.
