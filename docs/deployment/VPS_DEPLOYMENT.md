# Déployer Trezutil sur https://trezutil.com (VPS partagé avec Odoo)

Ce guide met Trezutil en production sur un VPS Linux qui fait déjà tourner Odoo, avec la configuration de référence
4 vCPU, 8 Go de RAM et 100 Go de SSD. Il ne touche jamais à Odoo :

- Trezutil n'écoute que sur `127.0.0.1:18080`.
- Le nginx déjà présent sur le serveur reçoit un bloc `server` de plus pour trezutil.com.
- Les ports d'Odoo (8069, 8072), le PostgreSQL de l'hôte (5432), ses données et sa configuration restent intacts.

## 1. Architecture

```
Internet ── 443 ── nginx (hôte, déjà là pour Odoo)
                     ├── odoo.…        → 127.0.0.1:8069   (inchangé)
                     └── trezutil.com  → 127.0.0.1:18080  → conteneurs Trezutil (projet docker « trezutil »)
                                                             api-gateway (interface web + /api/v1)
                                                             identity · conversation · search · order · payment
                                                             postgres (volume trezutil_pgdata, sans port publié)
                                                             opensearch (sans port publié)
```

Une seule image (`trezutil:latest`) contient les services Go et l'interface web compilée. Le service `migrate`
applique les migrations, uniquement en avant, avant le démarrage des autres services.

**Ressources.** Le fichier `docker-compose.vps.yml` plafonne la mémoire à environ 3,6 Go au total. La consommation
mesurée au repos est d'environ 1,2 Go, dont 1,05 Go pour OpenSearch. Il faut prévoir 5 Go de disque libres au
minimum, sans compter la croissance de la base et des sauvegardes.

## 2. DNS (chez le registrar de trezutil.com)

| Type | Nom | Valeur |
|---|---|---|
| A | `@` | adresse IPv4 du VPS |
| AAAA | `@` | adresse IPv6 du VPS (seulement si le VPS en a une, sinon ne pas créer) |
| CNAME | `www` | `trezutil.com.` |

Pour vérifier depuis n'importe quel poste : `dig +short trezutil.com` et `dig +short www.trezutil.com` doivent
afficher l'adresse du VPS. La propagation prend de quelques minutes à quelques heures.

## 3. Prérequis sur le VPS

- Docker Engine et son greffon compose, en version 2.24 ou plus : `docker compose version`.
- nginx (déjà installé pour Odoo) et certbot : `apt install certbot` si besoin.
- Les ports 80 et 443 ouverts vers nginx. Ne jamais ouvrir 18080 vers l'extérieur.
- `git`, `curl`.

### Docker rootless pour le compte `trezutil-deploy` (installé le 22/11/2026)

Trezutil tourne dans le Docker « rootless » du compte `trezutil-deploy`. Le compte n'a aucun droit root, et les
conteneurs ne peuvent pas agir sur l'hôte, ni sur Odoo, ni sur sa base. Le démon Docker système (root) est installé
mais désactivé : il ne touche pas au pare-feu et ne crée pas de pont `docker0`.

Ce qui a été fait, en root, pour mémoire ou pour refaire l'opération sur un autre serveur :

1. Création de `trezutil-deploy` : clé SSH seule, mot de passe inutilisable (`*`), sans sudo.
2. Dépôt officiel Docker (clé `9DC8 5822 9FC7 DD38 854A E2D8 8D81 803C 0EBF CD88`) et installation de `docker-ce`,
   `docker-ce-cli`, `containerd.io`, `docker-buildx-plugin`, `docker-compose-plugin`, `docker-ce-rootless-extras`,
   `uidmap` et `slirp4netns`. Pendant l'installation, `policy-rc.d` empêche tout démarrage de service, et
   `NEEDRESTART_MODE=l` empêche le redémarrage automatique d'Odoo.
3. `systemctl disable --now docker.service docker.socket containerd.service`.
4. Profil AppArmor `/etc/apparmor.d/usr.bin.rootlesskit`, chargé par `apparmor_parser -r`. Ubuntu 24.04 restreint
   les espaces de noms utilisateur.
5. `/etc/systemd/system/user@.service.d/delegate.conf` (`Delegate=cpu cpuset io memory pids`), puis
   `systemctl daemon-reload` : les plafonds CPU et mémoire du fichier compose s'appliquent.
6. `loginctl enable-linger trezutil-deploy` : le Docker du compte démarre au boot.

Puis, en tant que `trezutil-deploy` : `dockerd-rootless-setuptool.sh install`, `systemctl --user enable docker`,
`docker context use rootless`. Les commandes `docker` du compte visent son propre démon.

Avant toute installation, inventaire en lecture seule :

```bash
ss -ltnp                 # ports occupés (Odoo 8069/8072, PostgreSQL 5432, nginx 80/443)
free -m && df -h /       # mémoire et disque disponibles
ls /etc/nginx/sites-enabled/
```

## 4. Installation

```bash
# En tant que trezutil-deploy (ssh trezutil-vps) : le dépôt vit dans son répertoire, sans aucun droit root.
git clone <dépôt> ~/trezutil && cd ~/trezutil
cp deployments/docker/app.env.example deployments/docker/app.env
chmod 600 deployments/docker/app.env
```

Ensuite, remplir `deployments/docker/app.env`. Les secrets se génèrent avec `openssl rand -hex 32`, et n'entrent
jamais dans Git, car le fichier est ignoré.

| Variable | Valeur pour trezutil.com |
|---|---|
| `APP_ENV` | `production` |
| `POSTGRES_PASSWORD`, `JWT_SECRET`, `OTP_CODE_SECRET`, `METRICS_TOKEN` | secrets générés |
| `CORS_ALLOWED_ORIGINS` | `https://trezutil.com` |
| `TRUSTED_PROXY_HOPS` | `1` (derrière nginx) |
| `GATEWAY_LOCAL_PORT` | `18080` |
| `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` | les clés Stripe, pour activer le paiement en ligne |
| `AI_PROVIDER` et la clé du fournisseur | pour l'interprétation par modèle ; sans elles, le routage se fait par mots-clés |
| `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_OTP_TEMPLATE` | **obligatoires** : les codes de connexion partent par WhatsApp (Meta Cloud API, modèle d’authentification approuvé). Sans eux, personne ne peut se connecter en production |

## 5. Premier déploiement

```bash
bash scripts/ops/deploy.sh --check     # vérifications seules : configuration, ports, mémoire, disque
bash scripts/ops/deploy.sh             # sauvegarde, build, migrations, démarrage, contrôle de santé
```

Puis nginx et le certificat, en deux temps, car nginx refuse un bloc 443 tant que le certificat n'existe pas :

```bash
sudo cp deployments/nginx/trezutil-bootstrap.conf /etc/nginx/sites-available/trezutil.conf
sudo ln -s /etc/nginx/sites-available/trezutil.conf /etc/nginx/sites-enabled/trezutil.conf
sudo nginx -t && sudo systemctl reload nginx
sudo certbot certonly --webroot -w /var/www/html -d trezutil.com -d www.trezutil.com
sudo cp deployments/nginx/trezutil.conf /etc/nginx/sites-available/trezutil.conf
sudo nginx -t && sudo systemctl reload nginx
```

`reload` et non `restart` : Odoo garde ses connexions. Le renouvellement du certificat est automatique (timer
certbot) ; pour le tester : `sudo certbot renew --dry-run`.

Pour vérifier : `curl -I https://trezutil.com` doit répondre `200`, et `https://trezutil.com/healthz` doit renvoyer
`{"status":"ok"}`.

## 6. Mises à jour

```bash
cd ~/trezutil && git pull
bash scripts/ops/deploy.sh
```

Le script sauvegarde la base (`backups/`, avec vérification et somme de contrôle), garde l'image en cours sous le nom
`trezutil:previous`, reconstruit, migre, redémarre puis vérifie `/healthz` et `/readyz`.

## 7. Retour arrière

```bash
bash scripts/ops/rollback.sh           # remet l'image précédente (demande confirmation)
```

Le code revient en arrière, mais pas la base. Les migrations du dépôt sont uniquement additives, donc l'image
précédente fonctionne normalement sur le schéma migré. Si ce n'est pas le cas, il faut restaurer la sauvegarde prise
par `deploy.sh` juste avant la migration. C'est une étape manuelle et délibérée :

```bash
docker compose -p trezutil -f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml \
  --env-file deployments/docker/app.env stop api-gateway identity-service conversation-service search-service order-service payment-service
docker exec -i trezutil-postgres-1 pg_restore -U trezutil -d trezutil --clean --if-exists < backups/trezutil-<horodatage>.dump
bash scripts/ops/rollback.sh
```

## 8. Sauvegardes

Il faut une sauvegarde quotidienne, copiée hors du VPS : une sauvegarde sur le même disque n'en est pas une. Avec
crontab de `trezutil-deploy` (`crontab -e`) :

```
15 3 * * * cd $HOME/trezutil && PG_CONTAINER=trezutil-postgres-1 BACKUP_DIR=$HOME/trezutil/backups scripts/ops/backup.sh >> $HOME/trezutil-backup.log 2>&1
```

Une restauration se répète régulièrement : `scripts/ops/restore-check.sh backups/<dump>`. Le script restaure dans
une base jetable et compare les comptages.

## 9. Exploitation et diagnostic

| Besoin | Commande |
|---|---|
| État des services | `docker compose -p trezutil -f … ps` |
| Journaux d'un service | `docker compose -p trezutil -f … logs --tail 200 conversation-service` |
| Mémoire et CPU | `docker stats --no-stream` |
| Santé | `curl -s 127.0.0.1:18080/healthz` puis `/readyz` |
| Journaux nginx | `/var/log/nginx/trezutil.error.log` |
| Redémarrer un service | `docker compose -p trezutil -f … restart conversation-service` |

Ces commandes se lisent avec `-f deployments/docker/docker-compose.app.yml -f deployments/docker/docker-compose.vps.yml --env-file deployments/docker/app.env`.

| Symptôme | Piste |
|---|---|
| `502 Bad Gateway` sur trezutil.com | La gateway ne répond pas sur 18080 : `ps`, puis les logs de `api-gateway` |
| La gateway refuse de démarrer | `CORS_ALLOWED_ORIGINS` vide ou contenant `*` : en production, c'est volontaire |
| Connexion impossible | WhatsApp non configuré (`WHATSAPP_*`) : en production, le code n’est ni envoyé ni journalisé |
| OpenSearch redémarre en boucle | Mémoire : `docker stats`, plafond dans `docker-compose.vps.yml` |
| Paiement « non activé » | `STRIPE_SECRET_KEY` absent : le message affiché à l'utilisateur est attendu |

## 10. Ce qui n'est jamais fait automatiquement

- Aucune commande ne modifie Odoo, sa base, ses ports ou la configuration générale de nginx.
- Aucune migration destructive, et aucune restauration de base sans action manuelle.
- Aucun secret n'est versionné : `deployments/docker/app.env` est ignoré par Git.
