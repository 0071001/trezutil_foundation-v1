import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { LEGAL, type LegalDoc } from '@/lib/legal'

/*
 * The texts of the legal pages, in French. They describe what Trezutil really does (see docs/legal/LEGAL_CHECKLIST.md
 * for where each statement comes from in the product). A fact the code cannot know is never invented: it shows as a
 * highlighted marker until it is filled in src/lib/legal.ts.
 */

/** A fact that is not known yet: highlighted, never replaced by a made-up value. */
export function Missing({ what }: { what: string }) {
  return <mark className="legal-missing">[À compléter : {what}]</mark>
}

/** The fact when it is known, the marker otherwise. */
function F({ v, what }: { v: string; what: string }) {
  return v ? <>{v}</> : <Missing what={what} />
}

const E = LEGAL.entity
const Company = () => <><F v={E.name} what="nom de la société" />{E.form ? `, ${E.form}` : ''}</>
const Contact = () => <F v={E.email} what="adresse e-mail de contact" />

function Section({ title, children }: { title: string; children: ReactNode }) {
  return <section><h2>{title}</h2>{children}</section>
}

const terms = (
  <>
    <p className="legal-lede">Ces conditions s’appliquent à toute utilisation de Trezutil (site trezutil.com, assistant conversationnel, échanges par WhatsApp). En utilisant le service, tu les acceptes.</p>
    <Section title="1. Qui exploite Trezutil">
      <p>Trezutil est exploité par <Company />, dont le siège est situé <F v={E.address} what="adresse du siège" />. Contact : <Contact />.</p>
    </Section>
    <Section title="2. Ce que fait Trezutil">
      <p>Trezutil est un assistant commercial : tu exprimes ce que tu veux acheter, vendre ou faire réaliser, et Trezutil t’aide à trouver des fournisseurs, à comparer des offres, à organiser une opération en plusieurs étapes, puis à passer une commande, une réservation ou un paiement.</p>
      <p><b>Rien n’est commandé, réservé ou payé sans ta confirmation explicite.</b> Avant tout engagement, Trezutil te présente un récapitulatif que tu dois accepter ; sans cette confirmation, aucune action n’est exécutée.</p>
      <p>Trezutil n’est pas le vendeur des produits et services proposés par les fournisseurs : le contrat de vente ou de prestation est conclu entre toi et le fournisseur. <Missing what="qualification juridique exacte du rôle de Trezutil (plateforme, intermédiaire, mandataire) — à confirmer par un juriste" /></p>
    </Section>
    <Section title="3. Ton compte">
      <p>Tu peux utiliser Trezutil sans compte pour rechercher et comparer. Les commandes, réservations, paiements, la mémoire commerciale et l’historique synchronisé nécessitent un compte.</p>
      <p>Tu te connectes soit avec ton numéro de téléphone (un code à usage unique t’est envoyé sur WhatsApp ; pas de mot de passe), soit avec ton compte Google. La première connexion crée ton compte. Il faut avoir au moins <F v={LEGAL.minimumAge} what="âge minimum" /> pour ouvrir un compte.</p>
      <p>Tu es responsable de l’usage de ton compte et de ton téléphone ; ne communique jamais ton code de connexion. Trezutil ne te le demandera jamais.</p>
    </Section>
    <Section title="4. L’assistant et l’intelligence artificielle">
      <p>Pour comprendre tes demandes et rédiger ses réponses, Trezutil utilise des modèles d’intelligence artificielle fournis par des tiers (voir la politique de confidentialité). Les prix, offres, disponibilités et fournisseurs présentés proviennent des données de Trezutil et des fournisseurs, jamais d’une invention du modèle : une réponse qui affirmerait un chiffre non établi est écartée.</p>
      <p>L’assistant peut néanmoins se tromper dans sa compréhension ou sa formulation. <b>Vérifie les informations importantes avant toute décision commerciale.</b></p>
    </Section>
    <Section title="5. Commandes, réservations et paiements">
      <p>Une commande ou une réservation est créée uniquement après ta confirmation. Les paiements par carte sont traités par Stripe ; Trezutil ne reçoit ni ne conserve ton numéro de carte. Les annulations et remboursements suivent les <Link to="/legal/remboursements">règles d’annulation et de remboursement</Link>.</p>
    </Section>
    <Section title="6. Crédits d’utilisation">
      <p>Certaines actions peuvent consommer des crédits (par exemple une recherche, une vérification de disponibilité ou une rédaction par l’IA). Le coût de chaque action payante et ton solde sont affichés dans Paramètres → Crédits et usage. Les crédits sont d’abord réservés, puis consommés seulement si l’action aboutit ; ils sont libérés sinon. Les crédits personnels et ceux d’une organisation sont distincts. <Missing what="prix, validité et remboursabilité des crédits achetés" /></p>
    </Section>
    <Section title="7. Ce qui est interdit">
      <ul>
        <li>utiliser Trezutil pour une activité illégale, frauduleuse ou trompeuse ;</li>
        <li>proposer ou demander des biens ou services interdits : <F v={LEGAL.prohibitedItems} what="liste des biens et services interdits" /> ;</li>
        <li>tenter d’accéder au compte, aux conversations ou aux données d’un autre utilisateur, ou de contourner les limites techniques du service ;</li>
        <li>envoyer des contenus illicites, haineux, ou portant atteinte aux droits d’autrui ;</li>
        <li>essayer de faire agir l’assistant contre ses règles (par exemple pour lui faire contourner une confirmation).</li>
      </ul>
    </Section>
    <Section title="8. Fournisseurs">
      <p>Les fournisseurs inscrits sur Trezutil acceptent les <Link to="/legal/fournisseurs">conditions fournisseurs</Link>. L’inscription d’un fournisseur ne vaut pas vérification : seuls les fournisseurs portant la mention « vérifié » ont été contrôlés par l’équipe Trezutil, et ce contrôle ne garantit pas la qualité de chaque prestation.</p>
    </Section>
    <Section title="9. Responsabilité">
      <p>Trezutil met tout en œuvre pour que le service fonctionne et que les informations affichées soient exactes, sans pouvoir garantir une disponibilité permanente. Le fournisseur reste responsable des produits et services qu’il vend. <Missing what="étendue et limites de la responsabilité de Trezutil — à rédiger par un juriste" /></p>
    </Section>
    <Section title="10. Suspension et fin">
      <p>Tu peux cesser d’utiliser Trezutil à tout moment et demander la suppression de ton compte (voir la politique de confidentialité). Trezutil peut suspendre un compte qui enfreint ces conditions ; une commande déjà confirmée reste régie par ses propres règles.</p>
    </Section>
    <Section title="11. Modifications, droit applicable et litiges">
      <p>Ces conditions peuvent évoluer ; la date de mise à jour figure en haut de la page et tu seras informé des changements importants. Droit applicable et juridiction compétente : <F v={E.jurisdiction} what="droit applicable et tribunaux compétents" />. <Missing what="procédure de réclamation et de médiation" /></p>
    </Section>
  </>
)

const privacy = (
  <>
    <p className="legal-lede">Cette politique explique quelles données Trezutil traite, pourquoi, avec qui elles sont partagées, combien de temps elles sont conservées et quels sont tes droits.</p>
    <Section title="1. Responsable du traitement">
      <p><Company />, <F v={E.address} what="adresse du siège" />. Pour toute question sur tes données : <Contact />.</p>
    </Section>
    <Section title="2. Les données que nous traitons">
      <ul>
        <li><b>Compte</b> : numéro de téléphone (si tu te connectes par WhatsApp), nom affiché, adresse e-mail (si tu la renseignes ou si Google l’a vérifiée), rôles et statut du compte. Avec Google : l’identifiant de ton compte Google, ton nom et ton adresse e-mail vérifiée. Les codes de connexion ne sont conservés que sous forme d’empreinte, pour quelques minutes.</li>
        <li><b>Conversations</b> : tes messages, les réponses de Trezutil, les offres et résultats affichés, le titre que tu donnes à une conversation et son archivage.</li>
        <li><b>Mémoire commerciale</b> : uniquement ce que tu choisis d’ajouter (préférences, marques, budgets, contraintes, fournisseurs favoris, projets). Trezutil refuse d’y enregistrer des numéros de carte, codes, mots de passe ou coordonnées bancaires. Tu peux la désactiver ou l’effacer à tout moment.</li>
        <li><b>Opérations, commandes, réservations et paiements</b> : ce que tu commandes ou réserves, auprès de qui, les montants, statuts et références de paiement. Le numéro de ta carte est saisi chez Stripe et n’est jamais transmis à Trezutil.</li>
        <li><b>Crédits</b> : ton solde et l’historique des crédits réservés, consommés ou libérés.</li>
        <li><b>Fournisseurs</b> : pour une entreprise qui s’inscrit, ses informations (raison sociale, coordonnées, identifiants légaux, secteurs, capacités, zones desservies) et les documents envoyés (RCCM, identification nationale, NIF, pièce d’identité du responsable).</li>
        <li><b>Données techniques</b> : adresse IP (pour limiter les abus), identifiants de requête et journaux techniques. Les journaux ne contiennent ni mot de passe, ni jeton, ni code de connexion.</li>
        <li><b>Voix</b> : la dictée et le mode vocal utilisent la reconnaissance vocale de ton navigateur ; le texte reconnu est envoyé comme un message écrit. Trezutil n’enregistre pas ta voix.</li>
      </ul>
    </Section>
    <Section title="3. Pourquoi">
      <ul>
        <li>fournir le service : te connecter, comprendre tes demandes, rechercher et comparer, exécuter les actions que tu confirmes, synchroniser ton historique ;</li>
        <li>sécuriser le service : prévenir la fraude, limiter les tentatives abusives, enquêter sur un incident ;</li>
        <li>vérifier les fournisseurs qui s’inscrivent ;</li>
        <li>respecter nos obligations légales (comptabilité, réclamations).</li>
      </ul>
      <p>Trezutil ne vend pas tes données et ne les utilise pas pour de la publicité. <Missing what="base légale de chaque traitement (contrat, intérêt légitime, obligation légale, consentement) — à valider par un juriste" /></p>
    </Section>
    <Section title="4. Avec qui elles sont partagées">
      <ul>
        <li><b>Anthropic</b> (modèles Claude, États-Unis) : le texte de tes messages, le contexte de la conversation et, si tu l’as activée, ta mémoire commerciale, pour comprendre ta demande et rédiger la réponse. Sans fournisseur d’IA disponible, Trezutil fonctionne avec une méthode à base de règles, sans transmettre tes messages.</li>
        <li><b>Google</b> : si tu utilises « Continuer avec Google », Google t’authentifie et nous transmet ton identifiant, ton nom et ton adresse e-mail. Si le modèle Gemini est activé, il peut recevoir les mêmes données que celles décrites pour Anthropic.</li>
        <li><b>Meta (WhatsApp)</b> : ton numéro de téléphone et les messages échangés avec Trezutil sur WhatsApp, et les codes de connexion.</li>
        <li><b>Stripe</b> : les informations nécessaires à un paiement par carte.</li>
        <li><b>Les fournisseurs</b> auprès desquels tu commandes ou réserves : les informations nécessaires pour honorer ta commande (nom, contact, détails de la commande).</li>
        <li><b>Notre hébergeur</b> : {LEGAL.hosting}.</li>
      </ul>
      <p>Certains de ces prestataires traitent les données hors de la République démocratique du Congo. <Missing what="pays concernés et garanties encadrant ces transferts — à valider par un juriste" /></p>
    </Section>
    <Section title="5. Combien de temps">
      <p><F v={LEGAL.retention} what="durées de conservation des comptes, conversations, commandes, documents fournisseurs et journaux" /></p>
      <p>Tu peux supprimer toi-même une conversation (elle est alors effacée de nos serveurs, avec ses messages) et ta mémoire commerciale. Les commandes et réservations liées restent conservées pour leurs obligations propres.</p>
    </Section>
    <Section title="6. Sécurité">
      <p>Les échanges sont chiffrés (HTTPS). Les codes de connexion sont stockés sous forme d’empreinte et ne peuvent être utilisés qu’un nombre limité de fois. Chaque compte n’accède qu’à ses propres données ; les documents des fournisseurs ne sont visibles que par les responsables de l’entreprise et l’équipe Trezutil. Les bases de données sont sauvegardées avant chaque mise à jour du service.</p>
    </Section>
    <Section title="7. Tes droits">
      <p>Tu peux demander l’accès à tes données, leur rectification, leur suppression, t’opposer à un traitement ou en demander la portabilité en écrivant à <Contact />. Tu peux corriger ton nom et ton e-mail dans Paramètres, et supprimer tes conversations et ta mémoire commerciale directement dans l’application. La suppression complète d’un compte se fait aujourd’hui sur demande écrite. Tu peux aussi saisir l’autorité de protection des données : <F v={LEGAL.authority} what="autorité de protection des données compétente" />.</p>
    </Section>
    <Section title="8. Mineurs">
      <p>Trezutil n’est pas destiné aux personnes de moins de <F v={LEGAL.minimumAge} what="âge minimum" />.</p>
    </Section>
    <Section title="9. Cookies">
      <p>Voir <Link to="/legal/cookies">Cookies et stockage local</Link>.</p>
    </Section>
  </>
)

const cookies = (
  <>
    <p className="legal-lede">Trezutil n’utilise <b>aucun cookie publicitaire ni outil de mesure d’audience</b>. C’est pourquoi aucun bandeau de consentement n’est affiché.</p>
    <Section title="Ce que Trezutil enregistre sur ton appareil">
      <ul>
        <li>ta session (jetons de connexion), pour rester connecté ;</li>
        <li>une copie de tes conversations et de ta mémoire commerciale, pour les afficher immédiatement et garder une modification faite hors ligne jusqu’à la prochaine synchronisation ;</li>
        <li>tes préférences : langue, thème, profil choisi (personnel ou entreprise), notifications.</li>
      </ul>
      <p>Ces données restent dans le stockage local de ton navigateur. La déconnexion efface la session ; tu peux tout effacer depuis les réglages de ton navigateur.</p>
    </Section>
    <Section title="Services tiers">
      <ul>
        <li><b>Google</b> : si tu utilises « Continuer avec Google », le bouton est fourni par Google, qui peut déposer ses propres cookies pour t’authentifier.</li>
        <li><b>Stripe</b> : lors d’un paiement par carte, Stripe peut déposer des cookies nécessaires à la sécurité du paiement et à la prévention de la fraude.</li>
      </ul>
      <p>Si un outil de mesure d’audience était ajouté un jour, ton consentement serait demandé au préalable.</p>
    </Section>
  </>
)

const refunds = (
  <>
    <p className="legal-lede">Ces règles s’appliquent aux commandes et réservations passées via Trezutil.</p>
    <Section title="Annuler">
      <p>Tu peux annuler une commande ou une réservation depuis l’application (Achats et réservations) tant que son statut le permet. Une annulation n’est jamais faite à ta place sans ta demande.</p>
    </Section>
    <Section title="Être remboursé">
      <p>Un paiement par carte annulé ou refusé par le fournisseur est remboursé sur le moyen de paiement utilisé, via Stripe. Un remboursement ne peut jamais dépasser ce qui a été payé. Délai pour demander un remboursement : <F v={LEGAL.refundWindow} what="délai pour demander un remboursement" />. Délai avant de recevoir l’argent : <F v={LEGAL.refundDelay} what="délai de remboursement" />.</p>
    </Section>
    <Section title="Litige avec un fournisseur">
      <p>Le fournisseur est responsable du produit ou du service livré. En cas de problème, contacte d’abord le fournisseur depuis ta commande, puis l’équipe Trezutil à <Contact /> si aucune solution n’est trouvée. <Missing what="procédure de réclamation, conditions des produits non retournables et droit de rétractation applicable" /></p>
    </Section>
  </>
)

const suppliers = (
  <>
    <p className="legal-lede">Ces conditions s’appliquent aux entreprises et personnes qui s’inscrivent comme fournisseurs sur Trezutil, en plus des conditions générales d’utilisation.</p>
    <Section title="1. Inscription et vérification">
      <p>L’inscription est gratuite et se fait en ligne : la personne qui inscrit l’entreprise en devient le propriétaire sur Trezutil. Tu t’engages à fournir des informations exactes et des documents authentiques et à les tenir à jour.</p>
      <p>Trois étapes distinctes : <b>inscrit</b> (dossier en cours), <b>documents soumis</b> (en attente), <b>vérifié</b>. Seule l’équipe Trezutil décide de la vérification, après examen des documents ; elle peut demander des compléments ou refuser un dossier, en indiquant pourquoi. L’inscription ne vaut jamais vérification.</p>
    </Section>
    <Section title="2. Tes engagements">
      <ul>
        <li>proposer des prix, disponibilités et conditions réels, et honorer les offres et commandes que tu acceptes ;</li>
        <li>ne proposer aucun produit ou service interdit (<F v={LEGAL.prohibitedItems} what="liste des biens et services interdits" />), contrefait ou dangereux ;</li>
        <li>respecter les lois applicables à ton activité (licences, fiscalité, protection des consommateurs) ;</li>
        <li>utiliser les données des acheteurs uniquement pour traiter leurs commandes et réservations, jamais pour un autre usage ni pour les revendre.</li>
      </ul>
    </Section>
    <Section title="3. Demandes, négociations et commandes">
      <p>Trezutil peut te transmettre des demandes d’acheteurs et, lorsque l’acheteur l’y autorise, négocier en son nom dans les limites qu’il a fixées. Une proposition n’engage l’acheteur qu’après sa confirmation explicite. Tes réponses sont conservées dans l’historique de la négociation.</p>
    </Section>
    <Section title="4. Paiements et frais">
      <p>Frais Trezutil : <F v={LEGAL.supplierFees} what="commission ou abonnement facturé aux fournisseurs" />. <Missing what="modalités de paiement des fournisseurs (compte de paiement, délais de versement, KYC)" /></p>
    </Section>
    <Section title="5. Documents et données">
      <p>Les documents que tu envoies ne sont visibles que par les responsables de ton entreprise et l’équipe Trezutil, et servent uniquement à la vérification. Voir la <Link to="/legal/confidentialite">politique de confidentialité</Link>.</p>
    </Section>
    <Section title="6. Suspension">
      <p>Trezutil peut suspendre ou retirer un fournisseur qui fournit de fausses informations, ne respecte pas ses engagements ou fait l’objet de réclamations répétées, après l’en avoir informé sauf urgence.</p>
    </Section>
  </>
)

const notice = (
  <>
    <Section title="Éditeur">
      <p><Company /><br /><F v={E.address} what="adresse du siège" /><br /><F v={E.registration} what="RCCM, identification nationale, NIF" /><br />Contact : <Contact /> · <F v={E.phone} what="téléphone" /></p>
    </Section>
    <Section title="Directeur de la publication">
      <p><F v={E.director} what="nom du directeur de la publication" /></p>
    </Section>
    <Section title="Hébergement">
      <p>{LEGAL.hosting}</p>
    </Section>
    <Section title="Propriété intellectuelle">
      <p>La marque, le logo et l’interface de Trezutil appartiennent à <Company />. Les marques et logos des partenaires et fournisseurs appartiennent à leurs titulaires.</p>
    </Section>
  </>
)

export const LEGAL_CONTENT: Record<LegalDoc, ReactNode> = {
  conditions: terms,
  confidentialite: privacy,
  cookies,
  remboursements: refunds,
  fournisseurs: suppliers,
  mentions: notice,
}
