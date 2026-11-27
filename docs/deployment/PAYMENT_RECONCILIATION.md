# Réconciliation des paiements

Pour les personnes qui exploitent Trezutil. Aucune des commandes ci-dessous ne contacte un prestataire de paiement,
ne débite ou ne rembourse qui que ce soit.

## Principe

- Une capture ou un remboursement n'existe dans Trezutil qu'avec ses écritures comptables : les deux sont écrits dans la
  même transaction PostgreSQL. Si le registre ne peut pas être écrit, le paiement ne change pas ; le webhook du
  prestataire est redemandé, ou la lecture du statut reprend l'opération.
- Un résultat inconnu (délai dépassé, statut non reconnu) n'est jamais un succès : le remboursement reste `pending`.
- Une demande sans réponse n'est renvoyée au prestataire avec la même clé que pendant 23 heures. Au-delà, il peut avoir
  oublié la clé : la renvoyer risquerait de l'exécuter deux fois. Elle doit être vérifiée à la main.

## Passage automatique (service de paiement)

Toutes les `PAYMENT_RECONCILE_SECONDS` secondes (900 par défaut, `0` pour le désactiver), le service relit chez le
prestataire le statut des paiements récents (7 jours) restés sans réponse définitive, enregistre ce statut, puis écrit
un avertissement s'il reste des incohérences. Il ne corrige jamais le registre de lui-même.

## Rapport et correction

```sh
trezutil-admin reconcile                 # lecture seule ; code de sortie 3 s'il y a des constats
trezutil-admin reconcile -json
trezutil-admin repair-ledger -actor "prenom.nom"           # simulation : liste ce qui serait ajouté
trezutil-admin repair-ledger -actor "prenom.nom" -apply    # ajoute les lignes manquantes, tracé dans audit_events
```

`repair-ledger` ajoute uniquement les lignes manquantes d'une capture ou d'un remboursement réussi déjà enregistrés.
Il ne modifie et ne supprime rien.

| Constat | Signification | Action |
|---|---|---|
| `capture_without_ledger` | paiement capturé sans ses deux lignes | vérifier chez le prestataire, puis `repair-ledger` |
| `refund_without_ledger` | remboursement réussi sans ses deux lignes (les anciens remboursements n'avaient qu'une ligne) | `repair-ledger` |
| `duplicate_posting` | la même ligne écrite plusieurs fois | correction comptable décidée par une personne ; ne rien supprimer sans trace |
| `posting_amount_mismatch` | montant ou devise différents de l'enregistrement | vérifier chez le prestataire |
| `posting_without_capture` | lignes de capture sur un paiement non capturé | vérifier chez le prestataire : l'argent a-t-il été reçu ? |
| `refunded_total_mismatch` | total remboursé différent de la somme des remboursements réussis | vérifier chez le prestataire |
| `refund_pending` | remboursement sans réponse depuis plus d'une heure | procédure ci-dessous |
| `payment_stuck` | paiement sans réponse depuis plus d'une heure | vérifier chez le prestataire |
| `captured_order_unpaid` | argent reçu, commande toujours en attente | vérifier la commande ; rejouer le webhook |
| `webhook_unprocessed` | événement en échec, abandonné ou `review` (contredit l'enregistrement) | lire `error_message`, comparer avec le prestataire |

## Remboursement resté `pending`

1. Chercher le remboursement dans le tableau de bord du prestataire (référence, montant, date).
2. S'il y figure comme réussi : demander au prestataire de renvoyer le webhook ; Trezutil l'enregistre une seule fois.
3. S'il n'y figure pas après 23 heures : ne pas relancer avec la même clé. Documenter la vérification avant toute
   nouvelle demande. Il n'existe pas encore de commande pour clore manuellement un remboursement.

## SasPay

L'intégration ne dispose d'aucune API de remboursement, d'annulation ni de capture chez SasPay : une demande de
remboursement d'un paiement SasPay est refusée (`REFUND_REFUSED`). Un remboursement fait à la main dans le tableau de
bord SasPay n'est pas connu de Trezutil. Tant que ce point n'est pas tranché (voir
`docs/architecture/ADR_PAYMENT_COLLECTION_AND_SETTLEMENT.md`), chaque remboursement manuel doit être consigné hors de
Trezutil avec la référence du paiement.

Non vérifié : la durée pendant laquelle SasPay conserve une clé d'idempotence, et même qu'il la respecte. À confirmer
par écrit auprès de SasPay.

## Doublons antérieurs à la migration 000038

La migration crée l'index qui interdit les doubles écritures seulement si la base n'en contient aucun ; sinon elle
émet un avertissement et conserve toutes les données. Après correction par une personne, créer l'index avec la
définition de `migrations/000038_financial_integrity.sql`.
