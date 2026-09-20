# Security Model

## Principe

Toutes les opérations sur `Listing` doivent vérifier :

1. L’authentification de l’acteur.
2. L’autorisation selon le rôle.
3. L’isolation par `organization_id`.
4. L’ownership du `Seller` / `Store`.

## Rôles et permissions

### Platform Roles

```text
RoleAdmin
RoleOperator
RoleStaff
RoleModerator
```

### Organization Roles

```text
OrganizationRoleOwner
OrganizationRoleAdmin
OrganizationRoleManager
OrganizationRoleStaff
```

## Matrice

| Action | Platform Admin | Operator | Org Owner | Org Admin | Seller | Store Staff | Public |
|---|---|---|---|---|---|---|---|
| Créer Listing | ✅ | ✅ | ❌ | ❌ | ✅ (son seller) | ❌ | ❌ |
| Modifier son Listing | ❌ | ❌ | ❌ | ❌ | ✅ (son seller) | ✅ (son store) | ❌ |
| Modifier Listing d’un autre Seller | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Publier Listing | ❌ | ❌ | ✅ (son org) | ✅ (son org) | ✅ (son seller) | ❌ | ❌ |
| Suspendre Listing | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Voir Listing publié | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Voir Listing draft | ❌ | ❌ | ✅ (son org) | ✅ (son org) | ✅ (son seller) | ✅ (son store) | ❌ |
| Rechercher Listings | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

## Règles d’isolation

### Organization

* Un `Listing` appartient toujours à une `Organization` via son `Store`.
* Aucune requête ne doit retourner un `Listing` d’une autre `Organization` sans autorisation explicite.

### Seller

* Un `Seller` ne peut modifier que ses propres `Listing`s.
* Un `Seller` ne peut créer un `Listing` que dans un `Store` qui lui appartient.

### Store

* Un `StoreStaff` ne peut modifier que les `Listing`s du `Store` auquel il appartient.
* Un `StoreStaff` ne peut pas voir les `Listing`s d’autres `Store`s de la même `Organization` sauf si autorisé.

### Public

* Les listings `PUBLISHED` sont visibles publiquement.
* Les listings `DRAFT`, `PENDING_REVIEW`, `PAUSED` ne sont pas publics.
* Les listings `SOLD`, `CLOSED`, `EXPIRED` ne sont pas publics.

## Validation serveur

Règles :

* Le serveur dérive toujours `organization_id` depuis le `Store`.
* Le serveur vérifie toujours que l’acteur appartient à l’`Organization`.
* Le serveur vérifie toujours que l’acteur a la permission sur le `Seller` ou le `Store`.
* Le client ne fournit jamais `organization_id`, `seller_id`, `store_id` sans validation.

## Audit

* Toute création, modification, suppression de `Listing` doit être auditée.
* L’audit inclut :
  * actor_id
  * action
  * listing_id
  * organization_id
  * seller_id
  * store_id
  * timestamp
  * changes

## Modération

* Un `Listing` créé passe par `PENDING_REVIEW` si modération activée.
* `RoleModerator` peut :
  * approuver → `PUBLISHED`
  * rejeter → `DRAFT`
  * suspendre → `PAUSED`

## Confidentialité

* `UserLocation` n’est jamais exposée publiquement.
* Les métadonnées sensibles ne sont pas stockées dans `metadata` sans chiffrement.
* Les `Listing`s privés ne sont accessibles qu’à leur propriétaire et aux rôles autorisés.
