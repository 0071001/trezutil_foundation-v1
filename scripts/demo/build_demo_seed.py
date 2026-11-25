"""Build the Trezutil demo catalogue (scripts/demo/demo_seed.sql and scripts/demo/remove_demo.sql).

Sources:
  * Product demo/apparel.csv, home-and-garden.csv, jewelery.csv (Shopify sample exports; images from Shopify's free
    "Burst" library): translated into French and given to Kinshasa demo shops (the files only name placeholder vendors).
  * scripts/demo/data/kinshasa_seed.json (Trezutil demo dataset: phones and electronics, home services, restaurants,
    hotels, real estate; images from Unsplash). Each restaurant and each hotel becomes its own supplier.
  * A building-materials shop priced in Congolese francs (to test mobile money in CDF).
Left out: woocommerce-sample-products.csv (WooCommerce-branded items with lorem ipsum) and fashion.csv ("Example ...").

Every supplier is fictional, marked "(démo)", unverified (trust 0) and reachable on +243821714747 (the number used to
test payments). Every listing says it is a demonstration offer. IDs are UUIDv5 from stable keys, so running the SQL
again updates the same rows instead of duplicating them; remove_demo.sql deletes exactly these rows.

Usage: python scripts/demo/build_demo_seed.py "<path to Product demo>"
"""
import csv
import io
import json
import os
import re
import sys
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
NS = uuid.UUID("6b1f2c4e-2a51-4f7e-9a3c-7d2e9b0c1a55")  # Trezutil demo namespace
PHONE = "+243821714747"  # 0821714747
BATCH = "2026-11-25"
DEMO_NOTE = "Offre de démonstration : aucune livraison réelle."


def uid(*parts):
    return str(uuid.uuid5(NS, ":".join(parts)))


def q(v):
    if v is None:
        return "NULL"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return repr(v)
    return "'" + str(v).replace("'", "''") + "'"


def slugify(s):
    s = s.lower()
    for a, b in (("à", "a"), ("â", "a"), ("ä", "a"), ("é", "e"), ("è", "e"), ("ê", "e"), ("ë", "e"), ("î", "i"), ("ï", "i"),
                 ("ô", "o"), ("ö", "o"), ("ù", "u"), ("û", "u"), ("ü", "u"), ("ç", "c"), ("œ", "oe"), ("’", "-"), ("'", "-")):
        s = s.replace(a, b)
    return re.sub(r"[^a-z0-9]+", "-", s).strip("-")


# Approximate centres of the Kinshasa communes and neighbourhoods used (for the map and "near me").
PLACES = {
    "Gombe": (-4.3040, 15.3050), "Limete": (-4.3530, 15.3420), "Ngaliema": (-4.3550, 15.2450), "Kintambo": (-4.3330, 15.2760),
    "Bandalungwa": (-4.3480, 15.2780), "Kalamu": (-4.3460, 15.3100), "Lingwala": (-4.3260, 15.3000), "Masina": (-4.3830, 15.3900),
    "Kasa-Vubu": (-4.3400, 15.3000), "Mont-Ngafula": (-4.4300, 15.2700), "Binza": (-4.3700, 15.2550), "Kimwenza": (-4.4600, 15.2900),
    "Ma Campagne": (-4.3650, 15.2650), "Matonge": (-4.3390, 15.3130), "Barumbu": (-4.3150, 15.3210),
}


def place(name):
    name = name.replace("‑", "-").replace("Kasa‑Vubu", "Kasa-Vubu")
    return name if name in PLACES else "Gombe"


# --- categories --------------------------------------------------------------------------------------------------

CATEGORIES = [  # (slug, name, parent slug)
    ("mode", "Mode et vêtements", None), ("mode-femme", "Vêtements femme", "mode"), ("mode-homme", "Vêtements homme", "mode"),
    ("sacs-chaussures", "Sacs et chaussures", "mode"),
    ("bijoux", "Bijoux", None), ("bracelets", "Bracelets", "bijoux"), ("colliers", "Colliers", "bijoux"), ("boucles-oreilles", "Boucles d’oreilles", "bijoux"),
    ("maison-jardin", "Maison et jardin", None), ("mobilier", "Mobilier", "maison-jardin"), ("decoration", "Décoration et linge", "maison-jardin"),
    ("jardinage", "Jardinage", "maison-jardin"),
    ("materiaux-construction", "Matériaux de construction", None),
    ("electronique", "Électronique", None), ("telephones", "Téléphones et montres", "electronique"), ("informatique", "Informatique et impression", "electronique"),
    ("image-son", "Image, son et jeux", "electronique"), ("electromenager", "Électroménager", "electronique"),
    ("services", "Services", None), ("services-maison", "Maison et dépannage", "services"), ("services-numeriques", "Numérique et réparation", "services"),
    ("evenementiel", "Événementiel", "services"), ("services-personne", "Services à la personne et transport", "services"),
    ("restaurants", "Restaurants", None), ("hotels", "Hôtels", None),
    ("immobilier", "Immobilier", None), ("immobilier-location", "Location", "immobilier"), ("immobilier-vente", "Vente", "immobilier"),
]

# --- the imported Shopify samples, translated --------------------------------------------------------------------

FR = {
    # apparel: (title, description, category, shop)
    "ocean-blue-shirt": ("Chemise bleu océan", "Chemise en coton bleu océan, col étroit, boutonnée sur le devant, manches longues. Coupe confortable, motifs kaléidoscope.", "mode-homme", "gombe-gentleman"),
    "classic-varsity-top": ("Haut varsity classique", "Haut décontracté gris et noir boutonné, d’inspiration sportive, avec une lettre brodée.", "mode-femme", "matonge-fashion"),
    "yellow-wool-jumper": ("Pull en laine jaune", "Pull tricoté en laine douce, épaules tombantes, manches larges et poignets épais.", "mode-femme", "matonge-fashion"),
    "floral-white-top": ("Haut blanc fleuri", "Haut blanc sans manches à motif floral, élégant et léger.", "mode-femme", "matonge-fashion"),
    "striped-silk-blouse": ("Blouse en soie rayée", "Blouse en soie rayée noir et rouge avec col à boucle et pantalon assorti.", "mode-femme", "matonge-fashion"),
    "classic-leather-jacket": ("Veste en cuir classique", "Veste en cuir zippée pour femme, ceinture réglable, épaulettes et poche zippée.", "mode-femme", "matonge-fashion"),
    "dark-denim-top": ("Haut en denim foncé", "Haut en denim foncé avec poches poitrine, manches longues boutonnées et ourlet effet déchiré.", "mode-femme", "matonge-fashion"),
    "navy-sport-jacket": ("Veste de sport bleu marine", "Veste imperméable bleu marine à manches longues, polyester fin et doublure en maille douce.", "mode-homme", "gombe-gentleman"),
    "dark-winter-jacket": ("Veste matelassée noire", "Veste noire épaisse doublée de polaire, idéale pour les soirées fraîches et les voyages.", "mode-femme", "matonge-fashion"),
    "black-leather-bag": ("Sac en cuir noir", "Sac en cuir noir spacieux, à porter à l’épaule ou à la main.", "sacs-chaussures", "matonge-fashion"),
    "zipped-jacket": ("Veste zippée homme", "Veste imperméable bleu marine et bleu clair, avec poche poitrine zippée.", "mode-homme", "gombe-gentleman"),
    "silk-summer-top": ("Haut d’été en soie", "Haut en soie à manches courtes avec motif de chiffres.", "mode-femme", "matonge-fashion"),
    "longsleeve-cotton-top": ("Haut en coton manches longues", "Haut en coton noir à manches longues, sans col, ourlet épais.", "mode-femme", "matonge-fashion"),
    "chequered-red-shirt": ("Chemise à carreaux rouge", "Chemise en flanelle à carreaux, manches longues et deux poches poitrine.", "mode-homme", "gombe-gentleman"),
    "white-cotton-shirt": ("Chemise blanche en coton", "Chemise blanche en coton à manches longues, col souple, petits boutons et poche avant.", "mode-femme", "matonge-fashion"),
    "olive-green-jacket": ("Veste vert olive", "Veste ample vert olive avec boutons et grandes poches, motif coloré aux épaules.", "mode-femme", "matonge-fashion"),
    "blue-silk-tuxedo": ("Smoking en soie bleue", "Smoking en soie bleue à motif marbré, doublure sombre et boutons noirs.", "mode-homme", "gombe-gentleman"),
    "red-sports-tee": ("T-shirt de sport rouge", "T-shirt de sport rouge pour femme, détails colorés aux manches et petite poche blanche.", "mode-femme", "matonge-fashion"),
    "striped-skirt-and-top": ("Ensemble jupe rayée et haut", "Haut en coton noir avec jupe rayée assortie.", "mode-femme", "matonge-fashion"),
    "led-high-tops": ("Baskets montantes LED", "Baskets montantes noires avec semelle à LED vertes, lacets et boucle.", "sacs-chaussures", "gombe-gentleman"),
    # home and garden
    "clay-plant-pot": ("Pot en terre cuite", "Pot classique en terre cuite pour plantes d’intérieur ou de terrasse.", "jardinage", "jardin-ngaliema"),
    "copper-light": ("Lampe de chevet en cuivre", "Lampe de chevet élégante en cuivre.", "decoration", "maison-lingwala"),
    "cream-sofa": ("Canapé crème", "Canapé confortable crème sur base en bois.", "mobilier", "maison-lingwala"),
    "antique-drawers": ("Commode ancienne", "Commode ancienne en bois massif.", "mobilier", "maison-lingwala"),
    "white-bed-clothes": ("Parure de lit blanche", "Parure de lit blanche, sobre et douce.", "decoration", "maison-lingwala"),
    "pink-armchair": ("Fauteuil rose", "Fauteuil rose au design soigné.", "mobilier", "maison-lingwala"),
    "wooden-outdoor-table": ("Table de jardin en bois", "Table d’extérieur en bois pour jardin ou terrasse.", "jardinage", "jardin-ngaliema"),
    "brown-throw-pillows": ("Coussins marron", "Coussins décoratifs marron.", "decoration", "maison-lingwala"),
    "white-ceramic-pot": ("Pot en céramique blanche", "Pot de fleurs en céramique blanche fait main.", "jardinage", "jardin-ngaliema"),
    "yellow-watering-can": ("Arrosoir jaune", "Arrosoir vintage jaune vif.", "jardinage", "jardin-ngaliema"),
    "gardening-hand-trowel": ("Transplantoir de jardin", "Transplantoir en métal avec manche en bois.", "jardinage", "jardin-ngaliema"),
    "biodegradable-cardboard-pots": ("Pots biodégradables en carton", "Pots de semis biodégradables en carton, pour l’extérieur.", "jardinage", "jardin-ngaliema"),
    "grey-sofa": ("Canapé gris 4 places", "Grand canapé gris quatre places.", "mobilier", "maison-lingwala"),
    "wooden-outdoor-slats": ("Lames de bois extérieures", "Lames de clôture en bois pour l’extérieur.", "jardinage", "jardin-ngaliema"),
    "wooden-fence": ("Clôture de jardin en bois", "Clôture de jardin en bois.", "jardinage", "jardin-ngaliema"),
    "yellow-sofa": ("Canapé jaune 2 places", "Canapé deux places jaune sur pieds en bois.", "mobilier", "maison-lingwala"),
    "knitted-throw-pillows": ("Coussins tricotés", "Coussins tricotés main, plusieurs couleurs.", "decoration", "maison-lingwala"),
    "vanilla-candle": ("Bougie vanille", "Bougie parfumée à la vanille en pot.", "decoration", "maison-lingwala"),
    "black-bean-bag": ("Pouf poire noir", "Pouf poire en cuir noir.", "mobilier", "maison-lingwala"),
    "bedside-table": ("Table de chevet", "Table de chevet en bois.", "mobilier", "maison-lingwala"),
    # jewellery
    "chain-bracelet": ("Bracelet 7 chakras", "Bracelet de perles 7 chakras, bleu ou noir.", "bracelets", "bijouterie-bandal"),
    "leather-anchor": ("Bracelet ancre homme", "Bracelet en cuir noir avec ancre dorée ou argentée.", "bracelets", "bijouterie-bandal"),
    "bangle-bracelet": ("Bracelet jonc doré", "Bracelet jonc doré serti de pierres.", "bracelets", "bijouterie-bandal"),
    "bangle-bracelet-with-feathers": ("Bracelet jonc bohème", "Bracelet jonc doré bohème à pompons multicolores.", "bracelets", "bijouterie-bandal"),
    "boho-earrings": ("Boucles d’oreilles bohème", "Boucles d’oreilles boules turquoise sur crochets en or 14 carats.", "boucles-oreilles", "bijouterie-bandal"),
    "choker-with-bead": ("Ras-de-cou avec perle", "Ras-de-cou noir avec perle en or 14 carats.", "colliers", "bijouterie-bandal"),
    "choker-with-gold-pendant": ("Ras-de-cou pendentif doré", "Ras-de-cou en cuir noir avec pendentif en or jaune 14 carats.", "colliers", "bijouterie-bandal"),
    "choker-with-triangle": ("Ras-de-cou triangle", "Ras-de-cou noir avec pendentif triangle en argent.", "colliers", "bijouterie-bandal"),
    "dainty-gold-neclace": ("Collier doré délicat", "Collier doré délicat à deux pendentifs.", "colliers", "bijouterie-bandal"),
    "dreamcatcher-pendant-necklace": ("Collier attrape-rêves", "Collier attrape-rêves en perles turquoise, plumes en argent.", "colliers", "bijouterie-bandal"),
    "galaxy-earrings": ("Boucles d’oreilles galaxie", "Boucles d’oreilles galaxie, fermoirs en argent sterling.", "boucles-oreilles", "bijouterie-bandal"),
    "gemstone": ("Collier pierre fine", "Pendentif pierre fine sur chaîne en argent sterling, turquoise ou quartz.", "colliers", "bijouterie-bandal"),
    "gold-bird-necklace": ("Collier oiseau en or", "Collier délicat en or 14 carats, oiseau entre deux chaînes.", "colliers", "bijouterie-bandal"),
    "looped-earrings": ("Boucles d’oreilles éléphant", "Petites boucles d’oreilles éléphant en or 14 carats, détail opale.", "boucles-oreilles", "bijouterie-bandal"),
    "guardian-angel-earrings": ("Boucles d’oreilles ange gardien", "Boucles d’oreilles ange gardien en argent sterling.", "boucles-oreilles", "bijouterie-bandal"),
    "moon-charm-bracelet": ("Bracelet lune", "Bracelet d’amitié à chaîne en or 14 carats avec breloque lune.", "bracelets", "bijouterie-bandal"),
    "origami-crane-necklace": ("Collier grue origami", "Collier grue en origami en argent sterling.", "colliers", "bijouterie-bandal"),
    "pretty-gold-necklace": ("Collier or et turquoise", "Collier en or 14 carats et perles turquoise, double chaîne.", "colliers", "bijouterie-bandal"),
    "silver-threader-necklace": ("Collier fil d’argent", "Collier chaîne en argent sterling passant dans un anneau.", "colliers", "bijouterie-bandal"),
    "stylish-summer-neclace": ("Collier d’été bohème", "Collier bohème double chaîne dorée avec pendentif turquoise.", "colliers", "bijouterie-bandal"),
}

# Prices in the samples that make no sense for the item (corrected, and reported).
PRICE_FIX = {"grey-sofa": (449.99, 520.0)}

SHOPS = {  # key: (name, neighbourhood, description)
    "matonge-fashion": ("Matonge Fashion", "Matonge", "Boutique de mode femme à Matonge : prêt-à-porter, vestes et sacs."),
    "gombe-gentleman": ("Gombe Gentleman", "Gombe", "Mode homme à la Gombe : chemises, vestes et chaussures."),
    "maison-lingwala": ("Maison Lingwala Déco", "Lingwala", "Mobilier, décoration et linge de maison."),
    "jardin-ngaliema": ("Jardin de Ngaliema", "Ngaliema", "Pots, outils et aménagements de jardin."),
    "bijouterie-bandal": ("Bijouterie Bandal", "Bandalungwa", "Bijoux fantaisie et en or 14 carats : colliers, bracelets, boucles d’oreilles."),
    "quincaillerie-limete": ("Quincaillerie Limete", "Limete", "Matériaux de construction et quincaillerie, livraison sur chantier à Kinshasa."),
}

# Building materials, priced in Congolese francs (to test mobile money in CDF).
MATERIALS = [
    ("Ciment CPJ 42.5 – sac de 50 kg", "Ciment Portland composé pour maçonnerie et béton courant.", 42000),
    ("Fer à béton 12 mm – barre de 12 m", "Barre d’acier haute adhérence pour poteaux, poutres et dalles.", 38000),
    ("Tôle ondulée galvanisée – 3 m", "Tôle de toiture galvanisée, épaisseur 0,30 mm.", 28000),
    ("Sable fin de rivière – 1 m³", "Sable lavé pour mortier et enduits, livré sur chantier.", 70000),
    ("Gravier concassé – 1 m³", "Gravier 5/15 pour béton, livré sur chantier.", 90000),
    ("Briques en terre cuite – lot de 100", "Briques pleines pour murs porteurs et cloisons.", 60000),
    ("Peinture acrylique blanche – 20 L", "Peinture murale intérieur et extérieur, lessivable.", 150000),
    ("Clous de 3 pouces – 1 kg", "Clous en acier pour charpente et coffrage.", 8000),
]

KIN_CAT = {  # Kinshasa dataset: title keyword -> category
    "iPhone": "telephones", "Samsung Galaxy": "telephones", "Pixel": "telephones", "Apple Watch": "telephones",
    "MacBook": "informatique", "Dell": "informatique", "Imprimante": "informatique",
    "Sony": "image-son", "JBL": "image-son", "PlayStation": "image-son", "Nintendo": "image-son", "TV ": "image-son", "Canon": "image-son",
    "Réfrigérateur": "electromenager", "Climatiseur": "electromenager",
    "Plomberie": "services-maison", "Électricité": "services-maison", "Climatisation": "services-maison", "Nettoyage": "services-maison",
    "Peinture": "services-maison", "Déménagement": "services-maison", "groupe électrogène": "services-maison",
    "Réparation smartphones": "services-numeriques", "site web": "services-numeriques", "Montage vidéo": "services-numeriques",
    "Photographe": "evenementiel", "Traiteur": "evenementiel",
    "Barber": "services-personne", "Lavage automobile": "services-personne", "Livraison express": "services-personne",
}


def kin_category(item):
    v = item["vertical"]
    if v == "restaurant":
        return "restaurants"
    if v == "hotel":
        return "hotels"
    if v == "real_estate":
        return "immobilier-vente" if item.get("transaction_type") == "sale" else "immobilier-location"
    for k, c in KIN_CAT.items():
        if k in item["title"]:
            return c
    return "services-maison" if v == "service" else "electronique"


def main(demo_dir):
    out, rm = [], []
    W = out.append
    W("-- Trezutil demo catalogue (generated by scripts/demo/build_demo_seed.py; do not edit by hand).")
    W("-- Fictional suppliers marked (démo), unverified, phone " + PHONE + ". Re-running updates the same rows.")
    W("BEGIN;")

    # locations: Kinshasa, then the neighbourhoods
    city_id = uid("location", "Kinshasa")
    W(f"INSERT INTO locations (id, name, type, country, province, city, latitude, longitude, timezone) VALUES ({q(city_id)}, 'Kinshasa', 'city', 'CD', 'Kinshasa', 'Kinshasa', -4.32170000, 15.31250000, 'Africa/Kinshasa') ON CONFLICT (id) DO NOTHING;")
    loc = {}
    for name, (lat, lng) in PLACES.items():
        loc[name] = uid("location", "Kinshasa", name)
        W(f"INSERT INTO locations (id, parent_id, name, type, country, province, city, neighborhood, latitude, longitude, timezone) VALUES "
          f"({q(loc[name])}, {q(city_id)}, {q(name)}, 'neighborhood', 'CD', 'Kinshasa', 'Kinshasa', {q(name)}, {lat:.8f}, {lng:.8f}, 'Africa/Kinshasa') ON CONFLICT (id) DO NOTHING;")

    # categories (top level first)
    cat = {}
    for i, (slug, name, parent) in enumerate(CATEGORIES):
        cat[slug] = uid("category", slug)
        W(f"INSERT INTO categories (id, parent_id, name, slug, sort_order, status, scope) VALUES ({q(cat[slug])}, {q(cat[parent]) if parent else 'NULL'}, {q(name)}, {q(slug)}, {i}, 'active', 'global') "
          f"ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, parent_id = EXCLUDED.parent_id, updated_at = NOW();")

    suppliers = {}  # key -> (org, seller, store, neighbourhood)

    def supplier(key, name, hood, description):
        if key in suppliers:
            return suppliers[key]
        hood = place(hood)
        org, seller, store = uid("org", key), uid("seller", key), uid("store", key)
        label = f"{name} (démo)"
        desc = f"Fournisseur de démonstration Trezutil (fictif) : {description}"
        lat, lng = PLACES[hood]
        W(f"INSERT INTO organizations (id, type, legal_name, display_name, description, phone, email, country, province, city, address, status, verification_status) VALUES "
          f"({q(org)}, 'business', {q(label)}, {q(label)}, {q(desc)}, {q(PHONE)}, NULL, 'CD', 'Kinshasa', 'Kinshasa', {q(hood + ', Kinshasa')}, 'active', 'unverified') "
          f"ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, description = EXCLUDED.description, phone = EXCLUDED.phone, updated_at = NOW();")
        W(f"INSERT INTO seller_profiles (id, user_id, organization_id, seller_type, display_name, bio, phone, status, verification_status, trust_level) VALUES "
          f"({q(seller)}, NULL, {q(org)}, 'business', {q(label)}, {q(desc)}, {q(PHONE)}, 'active', 'unverified', 0) "
          f"ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, bio = EXCLUDED.bio, phone = EXCLUDED.phone, updated_at = NOW();")
        W(f"INSERT INTO stores (id, organization_id, name, slug, description, phone, country, province, city, address, latitude, longitude, status) VALUES "
          f"({q(store)}, {q(org)}, {q(label)}, {q('demo-' + slugify(key))}, {q(desc)}, {q(PHONE)}, 'CD', 'Kinshasa', 'Kinshasa', {q(hood + ', Kinshasa')}, {lat:.8f}, {lng:.8f}, 'active') "
          f"ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, phone = EXCLUDED.phone, updated_at = NOW();")
        suppliers[key] = (org, seller, store, hood)
        return suppliers[key]

    count = {"listings": 0}

    def listing(key, sup, title, description, category, vertical, ltype, price, currency, image=None, meta=None, compare_at=None):
        org, seller, store, hood = sup
        lid = uid("listing", key)
        md = {"demo": True, "demo_batch": BATCH}
        md.update(meta or {})
        if compare_at and compare_at > price:
            md["promotion"] = {"compare_at_amount": round(compare_at, 2)}
        W(f"INSERT INTO listings (id, organization_id, seller_id, store_id, category_id, title, description, listing_type, pricing_model, price_amount, price_currency, status, metadata, published_at, vertical) VALUES "
          f"({q(lid)}, {q(org)}, {q(seller)}, {q(store)}, {q(cat[category])}, {q(title)}, {q(DEMO_NOTE + ' ' + description)}, {q(ltype)}, 'fixed', {price:.2f}, {q(currency)}, 'published', "
          f"{q(json.dumps(md, ensure_ascii=False))}::jsonb, NOW(), {q(vertical)}) "
          f"ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description, category_id = EXCLUDED.category_id, price_amount = EXCLUDED.price_amount, "
          f"price_currency = EXCLUDED.price_currency, metadata = EXCLUDED.metadata, status = 'published', vertical = EXCLUDED.vertical, updated_at = NOW();")
        W(f"DELETE FROM listing_media WHERE listing_id = {q(lid)};")
        if image:
            W(f"INSERT INTO listing_media (id, listing_id, type, url, is_primary, sort_order) VALUES ({q(uid('media', key))}, {q(lid)}, 'image', {q(image)}, true, 0);")
        W(f"INSERT INTO listing_locations (id, listing_id, location_id, label, is_primary) VALUES ({q(uid('listing-location', key))}, {q(lid)}, {q(loc[hood])}, {q(hood + ', Kinshasa')}, true) ON CONFLICT (listing_id, location_id) DO NOTHING;")
        count["listings"] += 1

    # 1. the Shopify samples
    for f in ("apparel.csv", "home-and-garden.csv", "jewelery.csv"):
        rows = list(csv.DictReader(io.open(os.path.join(demo_dir, f), encoding="utf-8-sig")))
        first = {}
        for r in rows:
            h = r["Handle"]
            if h not in first and r["Title"].strip():
                first[h] = r
            if h in first and not first[h].get("Image Src") and r.get("Image Src"):
                first[h]["Image Src"] = r["Image Src"]
        for h, r in first.items():
            if h not in FR:
                raise SystemExit(f"no translation for {h}")
            title, desc, category, shop = FR[h]
            name, hood, sdesc = SHOPS[shop]
            price = float(r["Variant Price"] or 0)
            compare = float(r["Variant Compare At Price"]) if (r.get("Variant Compare At Price") or "").strip() else None
            if h in PRICE_FIX:
                price, compare = PRICE_FIX[h]
            listing("shopify:" + h, supplier(shop, name, hood, sdesc), title, desc, category, "RETAIL", "physical_product", price, "USD",
                    image=(r.get("Image Src") or "").strip() or None, meta={"source": "shopify-sample:" + f}, compare_at=compare)

    # 2. building materials in CDF
    name, hood, sdesc = SHOPS["quincaillerie-limete"]
    q_sup = supplier("quincaillerie-limete", name, hood, sdesc)
    for title, desc, price in MATERIALS:
        listing("materials:" + slugify(title), q_sup, title, desc, "materiaux-construction", "RETAIL", "physical_product", float(price), "CDF",
                meta={"source": "trezutil-demo", "delivery": "Livraison sur chantier à Kinshasa"})

    # 3. the Kinshasa dataset
    data = json.load(io.open(os.path.join(HERE, "data", "kinshasa_seed.json"), encoding="utf-8"))
    for it in data["items"]:
        v = it["vertical"]
        hood = place(it["location"]["neighborhood"])
        if v in ("restaurant", "hotel"):
            key, sname, sdesc = slugify(it["title"]), it["title"], it["description"]
        else:
            key = slugify(it["seller"]["name"])
            sname = it["seller"]["name"]
            sdesc = {"product": "Électronique et téléphonie à Kinshasa.", "service": "Services à domicile et prestations à Kinshasa.",
                     "real_estate": "Agence immobilière : location et vente à Kinshasa."}[v]
        sup = supplier(v + ":" + key, sname, hood, sdesc)
        vertical, ltype = {"product": ("RETAIL", "physical_product"), "service": ("SERVICES", "service"), "restaurant": ("RESTAURANT", "food"),
                           "hotel": ("HOTEL", "other"), "real_estate": ("REAL_ESTATE", "real_estate")}[v]
        meta = {"source": "trezutil-demo-kinshasa", "price_unit": it["price"].get("unit"), "tags": it.get("tags", [])}
        for k in ("attributes", "amenities", "rooms", "stars", "cuisine", "opening_hours", "bedrooms", "bathrooms", "transaction_type", "features", "rating", "service_area"):
            if k in it:
                meta[k] = it[k]
        image = it["images"][0]["url"] if it.get("images") else None
        listing("kinshasa:" + it["id"], sup, it["title"].replace("‑", "-"), it["description"], kin_category(it), vertical, ltype,
                float(it["price"]["amount"]), it["price"]["currency"], image=image, meta=meta)

    W("COMMIT;")

    # removal script: exactly these rows
    rm.append("-- Removes the Trezutil demo catalogue created by demo_seed.sql (and nothing else).")
    rm.append("BEGIN;")
    org_ids = ", ".join(q(s[0]) for s in suppliers.values())
    rm.append(f"DELETE FROM listings WHERE organization_id IN ({org_ids}) AND metadata->>'demo' = 'true';")
    rm.append(f"DELETE FROM stores WHERE id IN ({', '.join(q(s[2]) for s in suppliers.values())});")
    rm.append(f"DELETE FROM seller_profiles WHERE id IN ({', '.join(q(s[1]) for s in suppliers.values())});")
    rm.append(f"DELETE FROM organizations WHERE id IN ({org_ids}) AND display_name LIKE '%(démo)';")
    rm.append(f"DELETE FROM categories WHERE id IN ({', '.join(q(c) for c in cat.values())}) AND NOT EXISTS (SELECT 1 FROM listings WHERE listings.category_id = categories.id);")
    rm.append("COMMIT;")

    io.open(os.path.join(HERE, "demo_seed.sql"), "w", encoding="utf-8", newline="\n").write("\n".join(out) + "\n")
    io.open(os.path.join(HERE, "remove_demo.sql"), "w", encoding="utf-8", newline="\n").write("\n".join(rm) + "\n")
    print(f"suppliers: {len(suppliers)}, listings: {count['listings']}, categories: {len(cat)}")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, "..", "..", "Product demo"))
