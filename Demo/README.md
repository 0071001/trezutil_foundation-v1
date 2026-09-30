# Trezutil — Demo Commerce Catalog

Pack de données de démonstration destiné au frontend Trezutil / Kilo Code.

## Contenu

- Products: 15
- Services: 15
- Restaurants: 15
- Hotels: 15
- Immobilier: 15
- Total: 75

## Important

Les fiches marchandes sont **fictives** et créées pour la démonstration UI : elles ne doivent pas être présentées comme des commerçants vérifiés.

Les photos sont de vraies photographies externes chargées via des URLs LoremFlickr/Flickr. Elles ne sont pas des images générées par IA. Pour la production, remplacer les URLs par des médias appartenant à Trezutil ou correctement licenciés.

## Structure

`data/trezutil_demo_seed.json` est le fichier principal recommandé. Il contient les 75 listings.

Fichiers séparés :
- `products.json` / `products.csv`
- `services.json` / `services.csv`
- `restaurants.json` / `restaurants.csv`
- `hotels.json` / `hotels.csv`
- `real_estate.json` / `real_estate.csv`
- `image_manifest.json`

## Utilisation dans Kilo Code

1. Copier le dossier dans le workspace frontend/backend de Trezutil.
2. Inspecter le modèle réel de `Listing` et les contrats API existants avant import.
3. Faire un mapping vers les champs backend existants plutôt que modifier le backend uniquement pour ce seed.
4. Utiliser `demo=true` pour identifier clairement ces données.
5. Ne jamais remplacer silencieusement une erreur API réelle par ces données.

## Focus UX

Les données contiennent volontairement : prix, localisation, vendeur, disponibilité, images, tags, horaires, capacités et attributs afin d'alimenter :

- homepage
- nearby
- discover
- search
- conversation cards
- product/service/property detail
- restaurants
- hotels
- reservations
- seller views
