import type { ReactNode } from 'react'
import { ActionLink } from '@/components/ui'
import { LEGAL, type LegalDoc } from '@/lib/legal'

/*
 * The legal texts, in French and in English, written from what the product
 * really does (what it collects, who receives it, how orders, bookings and
 * refunds work). What only the company can say (its identity, retention
 * periods, refund windows, the governing law) is a visible marker, never an
 * invented value; and the whole text is a draft until a lawyer has reviewed it
 * (src/lib/legal.ts, docs/legal/LEGAL_CHECKLIST.md).
 */

export type LegalLang = 'fr' | 'en'

export interface LegalSection { id: string; title: string; body: ReactNode }
export interface LegalText { title: string; intro: string; sections: LegalSection[] }

const missing = (lang: LegalLang) => (label: string) => <mark className="legal-todo">{lang === 'fr' ? `[À compléter : ${label}]` : `[To complete: ${label}]`}</mark>

/** A fact from the legal configuration, or the visible marker that it is missing. */
const fact = (lang: LegalLang) => (value: string, label: string): ReactNode => (value ? value : missing(lang)(label))

const P = ({ children }: { children: ReactNode }) => <p>{children}</p>
const UL = ({ items }: { items: ReactNode[] }) => <ul>{items.map((item, i) => <li key={i}>{item}</li>)}</ul>

function contact(lang: LegalLang): ReactNode {
  const f = fact(lang)
  const { email, phone } = LEGAL.entity
  return lang === 'fr'
    ? <P>Écrivez-nous à {f(email, 'adresse e-mail de contact')}{phone ? ` ou appelez le ${phone}` : <> (téléphone : {f(phone, 'numéro de téléphone')})</>}.</P>
    : <P>Write to us at {f(email, 'contact email address')}{phone ? ` or call ${phone}` : <> (phone: {f(phone, 'phone number')})</>}.</P>
}

/* -------------------------------------------------------------- terms */

function terms(lang: LegalLang): LegalText {
  const f = fact(lang)
  const e = LEGAL.entity
  if (lang === 'fr') {
    return {
      title: 'Conditions d’utilisation',
      intro: 'Ces conditions disent ce que vous pouvez attendre de Trezutil et ce que nous attendons de vous. Elles s’appliquent au site web de Trezutil et à sa conversation sur WhatsApp.',
      sections: [
        { id: 'who', title: 'Qui nous sommes', body: <>
          <P>Trezutil est un service exploité par {f(e.name, 'nom de la société')} ({f(e.form, 'forme juridique')}), {f(e.address, 'adresse')}. Il aide chacun à trouver ce dont il a besoin (un logement, un produit, un service, une table) auprès de commerces et de prestataires indépendants, puis à commander, réserver et payer.</P>
          <P><strong>Trezutil n’est pas le vendeur.</strong> Le contrat de vente ou de prestation est conclu entre vous et le commerce ou le prestataire (le « vendeur ») dont vous choisissez l’offre.</P>
        </> },
        { id: 'account', title: 'Votre compte', body: <UL items={[
          'Vous vous connectez avec votre numéro de téléphone : un code à usage unique vous est envoyé sur WhatsApp. Utilisez votre propre numéro et gardez l’accès à votre téléphone : ce qui est fait avec votre compte est considéré comme fait par vous.',
          'Les informations que vous donnez doivent être exactes.',
          <>Vous devez avoir au moins {f(LEGAL.minimumAge, 'âge minimum')}.</>,
          'Vous pouvez cesser d’utiliser Trezutil à tout moment.',
        ]} /> },
        { id: 'assistant', title: 'L’assistant', body: <>
          <P>Trezutil comprend ce que vous écrivez et vous propose des offres. Il peut se tromper. <strong>Aucune commande ni réservation n’est faite sans que vous l’ayez confirmée expressément</strong> : vérifiez toujours le prix, la date et le vendeur avant de confirmer.</P>
          <P>L’assistant ne remplace ni la fiche de l’offre ni les conditions du vendeur.</P>
        </> },
        { id: 'orders', title: 'Commandes, réservations et paiements', body: <UL items={[
          'Le prix affiché est celui que le vendeur facture. Une promotion est une vraie réduction du prix du vendeur.',
          'Une commande est passée auprès du vendeur, qui la confirme. S’il ne peut pas la satisfaire, il peut la refuser ou l’annuler ; le remboursement d’une commande déjà payée suit la page Remboursements.',
          'Une réservation est acceptée selon la disponibilité au moment où vous réservez.',
          'Le paiement par carte est traité par un prestataire de paiement sécurisé (Stripe). Les numéros de carte sont saisis chez lui : Trezutil ne les reçoit ni ne les conserve.',
          'La livraison du produit ou la réalisation du service sont assurées par le vendeur.',
          'Les fonctions annoncées « bientôt » (contacter le vendeur, demander un devis) ne sont pas encore disponibles.',
        ]} /> },
        { id: 'sellers', title: 'Si vous vendez sur Trezutil', body: <>
          <P>L’ouverture d’un espace vendeur est assistée par l’équipe Trezutil. Le vendeur s’engage à :</P>
          <UL items={[
            'publier des offres exactes : prix, disponibilité, photos, lieu ;',
            'honorer les commandes et les réservations qu’il confirme, et tenir ses disponibilités à jour ;',
            'respecter la loi applicable à son activité ;',
            <>ne pas proposer {f(LEGAL.prohibitedItems, 'liste des produits et services interdits')}.</>,
          ]} />
        </> },
        { id: 'conduct', title: 'Usage correct', body: <UL items={[
          'Pas de produit, de service ou de contenu illégal, ni de fraude ou d’usurpation d’identité.',
          'Pas de harcèlement ni de propos menaçants.',
          'Pas d’extraction automatisée de données, ni de tentative de contourner la sécurité ou d’accéder aux données d’autrui.',
        ]} /> },
        { id: 'suspension', title: 'Suspension', body: <>
          <P>Nous pouvons suspendre un compte ou retirer une offre qui ne respecte pas ces conditions ou la loi, ou qui met d’autres personnes en danger. Si vous pensez qu’une décision est une erreur, écrivez-nous : nous la réexaminons.</P>
        </> },
        { id: 'availability', title: 'Disponibilité du service', body: <P>Trezutil est fourni tel quel. Il peut être interrompu pour maintenance ou en cas de panne, et ses fonctions évoluent. La conversation sur WhatsApp dépend de WhatsApp (Meta) et des opérateurs de téléphonie : nous ne maîtrisons pas leur disponibilité.</P> },
        { id: 'liability', title: 'Responsabilité', body: <>
          <P>Nous ne sommes pas responsables de la qualité, de la conformité ni de la livraison des produits et des services des vendeurs, sauf si la loi l’exige.</P>
          <P>{f('', 'étendue de la limitation de responsabilité, à définir avec le conseil juridique')}</P>
        </> },
        { id: 'changes', title: 'Modification des conditions', body: <P>Nous pouvons modifier ces conditions. La date de la dernière version figure en haut de la page ; un changement important est annoncé sur le site avant de s’appliquer.</P> },
        { id: 'law', title: 'Droit applicable et litiges', body: <P>{f(e.jurisdiction, 'droit applicable et juridiction compétente')}</P> },
        { id: 'contact', title: 'Nous contacter', body: contact(lang) },
      ],
    }
  }
  return {
    title: 'Terms of use',
    intro: 'These terms say what you can expect from Trezutil and what we expect from you. They apply to the Trezutil website and to its conversation on WhatsApp.',
    sections: [
      { id: 'who', title: 'Who we are', body: <>
        <P>Trezutil is a service operated by {f(e.name, 'company name')} ({f(e.form, 'legal form')}), {f(e.address, 'address')}. It helps people find what they need (a place to live, something to buy, a service, a table) from independent businesses and providers, and then order, book and pay.</P>
        <P><strong>Trezutil is not the seller.</strong> The contract of sale or service is between you and the business or provider (the “seller”) whose offer you choose.</P>
      </> },
      { id: 'account', title: 'Your account', body: <UL items={[
        'You sign in with your phone number: a one-time code is sent to you on WhatsApp. Use your own number and keep access to your phone: what is done with your account is treated as done by you.',
        'The information you give must be accurate.',
        <>You must be at least {f(LEGAL.minimumAge, 'minimum age')}.</>,
        'You can stop using Trezutil at any time.',
      ]} /> },
      { id: 'assistant', title: 'The assistant', body: <>
        <P>Trezutil understands what you write and suggests offers. It can be wrong. <strong>No order or booking is made unless you have expressly confirmed it</strong>: always check the price, the date and the seller before confirming.</P>
        <P>The assistant does not replace the offer’s page or the seller’s conditions.</P>
      </> },
      { id: 'orders', title: 'Orders, bookings and payments', body: <UL items={[
        'The price shown is the price the seller charges. A promotion is a real reduction of the seller’s price.',
        'An order is placed with the seller, who confirms it. If the seller cannot fulfil it, it may refuse or cancel it; the refund of an order already paid follows the Refunds page.',
        'A booking is accepted according to availability at the time you book.',
        'Card payments are processed by a secure payment provider (Stripe). Card numbers are entered with it: Trezutil neither receives nor keeps them.',
        'Delivery of the product or performance of the service is the seller’s responsibility.',
        'Features announced as “soon” (contacting the seller, requesting a quote) are not available yet.',
      ]} /> },
      { id: 'sellers', title: 'If you sell on Trezutil', body: <>
        <P>Opening a seller space is assisted by the Trezutil team. The seller undertakes to:</P>
        <UL items={[
          'publish accurate offers: price, availability, photos, location;',
          'honour the orders and bookings it confirms, and keep its availability up to date;',
          'comply with the law that applies to its activity;',
          <>not offer {f(LEGAL.prohibitedItems, 'list of prohibited products and services')}.</>,
        ]} />
      </> },
      { id: 'conduct', title: 'Proper use', body: <UL items={[
        'No illegal product, service or content, and no fraud or impersonation.',
        'No harassment or threats.',
        'No automated extraction of data, and no attempt to bypass security or reach other people’s data.',
      ]} /> },
      { id: 'suspension', title: 'Suspension', body: <P>We may suspend an account or remove an offer that breaks these terms or the law, or that puts other people at risk. If you think a decision is a mistake, write to us: we will look at it again.</P> },
      { id: 'availability', title: 'Availability of the service', body: <P>Trezutil is provided as it is. It may be interrupted for maintenance or in case of failure, and its features change. The conversation on WhatsApp depends on WhatsApp (Meta) and on telephone operators: we do not control their availability.</P> },
      { id: 'liability', title: 'Liability', body: <>
        <P>We are not responsible for the quality, conformity or delivery of sellers’ products and services, unless the law requires it.</P>
        <P>{f('', 'extent of the limitation of liability, to be defined with legal counsel')}</P>
      </> },
      { id: 'changes', title: 'Changes to these terms', body: <P>We may change these terms. The date of the latest version is at the top of the page; an important change is announced on the site before it applies.</P> },
      { id: 'law', title: 'Governing law and disputes', body: <P>{f(e.jurisdiction, 'governing law and competent courts')}</P> },
      { id: 'contact', title: 'Contact us', body: contact(lang) },
    ],
  }
}

/* ------------------------------------------------------------ privacy */

function privacy(lang: LegalLang): LegalText {
  const f = fact(lang)
  const e = LEGAL.entity
  if (lang === 'fr') {
    return {
      title: 'Politique de confidentialité',
      intro: 'Cette page explique quelles données Trezutil traite, pourquoi, avec qui elles sont partagées, combien de temps elles sont conservées et comment exercer vos droits.',
      sections: [
        { id: 'controller', title: 'Qui est responsable de vos données', body: <P>{f(e.name, 'nom de la société')}, {f(e.address, 'adresse')}. Pour toute question sur vos données : {f(e.email, 'adresse e-mail de contact')}.</P> },
        { id: 'collected', title: 'Les données que nous traitons', body: <UL items={[
          'Votre numéro de téléphone et, si vous les donnez, votre nom et votre adresse e-mail : pour vous connecter et vous reconnaître.',
          'Vos conversations avec Trezutil, sur le site ou sur WhatsApp : les messages que vous écrivez, ce que Trezutil en a compris et les résultats montrés, pour vous répondre et reprendre la conversation.',
          'Vos commandes et réservations : offres choisies, quantités, dates, montants, état, et références de paiement.',
          'Le paiement : le prestataire de paiement traite votre carte ; nous ne recevons que l’état du paiement et une référence.',
          'Si vous vendez : les informations de votre entreprise (nom, adresse, téléphone, e-mail, offres, photos).',
          'Des données techniques : journaux de sécurité et de fonctionnement (adresse IP, heure, identifiant de requête), pour protéger le service et corriger les pannes.',
        ]} /> },
        { id: 'use', title: 'Pourquoi nous les utilisons', body: <>
          <UL items={[
            'Fournir le service : vous connecter, trouver des offres, passer des commandes et des réservations, payer.',
            'Protéger le service et ses utilisateurs : sécurité, prévention de la fraude, limitation des abus.',
            'Respecter nos obligations légales.',
          ]} />
          <P>À ce jour, nous n’utilisons pas vos données pour de la publicité ciblée et nous ne les vendons pas.</P>
        </> },
        { id: 'bases', title: 'Sur quel fondement', body: <P>{f('', 'bases légales de chaque traitement, à définir avec le conseil juridique')}</P> },
        { id: 'recipients', title: 'Qui reçoit vos données', body: <UL items={[
          'Les vendeurs auprès de qui vous commandez ou réservez : ils reçoivent ce qu’il faut pour honorer la commande.',
          'WhatsApp (Meta), pour vous envoyer le code de connexion et échanger avec vous sur WhatsApp.',
          'Notre prestataire de paiement (Stripe).',
          'Des fournisseurs de modèles d’intelligence artificielle (parmi ceux que nous configurons, Anthropic ou Google), qui reçoivent le texte de votre message pour en comprendre l’intention. Sans fournisseur configuré, une méthode à règles est utilisée sur nos serveurs.',
          <>Notre hébergeur : {f(e.hosting, 'hébergeur et pays')}.</>,
          'Les autorités, lorsque la loi l’exige.',
        ]} /> },
        { id: 'transfers', title: 'Transferts hors du pays', body: <P>{f('', 'pays où les données sont hébergées et traitées, et garanties pour les transferts')}</P> },
        { id: 'retention', title: 'Combien de temps nous les gardons', body: <P>{f(LEGAL.retention, 'durées de conservation par type de données')}</P> },
        { id: 'security', title: 'Comment nous les protégeons', body: <UL items={[
          'Les échanges avec le site sont chiffrés (HTTPS).',
          'Les codes de connexion ne sont conservés que sous forme d’empreinte, expirent vite et ne tolèrent que quelques essais.',
          'Chaque service vérifie votre identité par un jeton signé, et les données sont cloisonnées par entreprise.',
          'Les journaux ne contiennent ni mot de passe, ni jeton, ni code de connexion.',
          'Aucun numéro de carte n’est conservé chez nous.',
        ]} /> },
        { id: 'rights', title: 'Vos droits', body: <>
          <P>Vous pouvez demander l’accès à vos données, leur correction, leur suppression ou vous opposer à un traitement. Écrivez à {f(e.email, 'adresse e-mail de contact')} depuis le numéro de votre compte, pour que nous puissions vous reconnaître.</P>
          <P>La suppression de compte en libre-service n’existe pas encore : la demande se fait par ce moyen.</P>
        </> },
        { id: 'storage', title: 'Cookies et stockage local', body: <P>Nous utilisons le stockage de votre navigateur, sans cookie publicitaire, pour garder votre session ouverte, mémoriser votre langue et votre thème, et conserver votre panier et vos favoris sur cet appareil. À ce jour nous n’utilisons ni outil de mesure d’audience ni traceur publicitaire.</P> },
        { id: 'children', title: 'Mineurs', body: <P>Trezutil est destiné aux personnes d’au moins {f(LEGAL.minimumAge, 'âge minimum')}.</P> },
        { id: 'changes', title: 'Modification de cette page', body: <P>La date de la dernière version figure en haut de la page ; un changement important est annoncé sur le site avant de s’appliquer.</P> },
        { id: 'authority', title: 'Réclamation', body: <P>Vous pouvez aussi vous adresser à : {f(LEGAL.authority, 'autorité de protection des données compétente')}.</P> },
      ],
    }
  }
  return {
    title: 'Privacy policy',
    intro: 'This page explains which data Trezutil processes, why, who it is shared with, how long it is kept and how to exercise your rights.',
    sections: [
      { id: 'controller', title: 'Who is responsible for your data', body: <P>{f(e.name, 'company name')}, {f(e.address, 'address')}. For any question about your data: {f(e.email, 'contact email address')}.</P> },
      { id: 'collected', title: 'The data we process', body: <UL items={[
        'Your phone number and, if you give them, your name and email address: to sign you in and recognise you.',
        'Your conversations with Trezutil, on the site or on WhatsApp: the messages you write, what Trezutil understood from them and the results shown, to answer you and resume the conversation.',
        'Your orders and bookings: chosen offers, quantities, dates, amounts, status, and payment references.',
        'Payment: the payment provider processes your card; we only receive the payment status and a reference.',
        'If you sell: your business’s information (name, address, phone, email, offers, photos).',
        'Technical data: security and operating logs (IP address, time, request identifier), to protect the service and fix failures.',
      ]} /> },
      { id: 'use', title: 'Why we use it', body: <>
        <UL items={[
          'To provide the service: sign you in, find offers, place orders and bookings, pay.',
          'To protect the service and its users: security, fraud prevention, limiting abuse.',
          'To meet our legal obligations.',
        ]} />
        <P>To date, we do not use your data for targeted advertising and we do not sell it.</P>
      </> },
      { id: 'bases', title: 'On what basis', body: <P>{f('', 'legal basis of each processing, to be defined with legal counsel')}</P> },
      { id: 'recipients', title: 'Who receives your data', body: <UL items={[
        'The sellers you order from or book with: they receive what is needed to honour the order.',
        'WhatsApp (Meta), to send you the sign-in code and to talk with you on WhatsApp.',
        'Our payment provider (Stripe).',
        'Artificial intelligence model providers (among those we configure, Anthropic or Google), who receive the text of your message to understand its intent. With no provider configured, a rule-based method is used on our servers.',
        <>Our host: {f(e.hosting, 'host and country')}.</>,
        'Authorities, where the law requires it.',
      ]} /> },
      { id: 'transfers', title: 'Transfers outside the country', body: <P>{f('', 'countries where data is hosted and processed, and safeguards for transfers')}</P> },
      { id: 'retention', title: 'How long we keep it', body: <P>{f(LEGAL.retention, 'retention periods by type of data')}</P> },
      { id: 'security', title: 'How we protect it', body: <UL items={[
        'Exchanges with the site are encrypted (HTTPS).',
        'Sign-in codes are kept only as a fingerprint, expire quickly and allow only a few attempts.',
        'Each service checks your identity with a signed token, and data is separated by business.',
        'Logs contain no password, token or sign-in code.',
        'No card number is kept by us.',
      ]} /> },
      { id: 'rights', title: 'Your rights', body: <>
        <P>You may ask for access to your data, its correction or deletion, or object to a processing. Write to {f(e.email, 'contact email address')} from your account’s number, so that we can recognise you.</P>
        <P>Self-service account deletion does not exist yet: the request is made this way.</P>
      </> },
      { id: 'storage', title: 'Cookies and local storage', body: <P>We use your browser’s storage, with no advertising cookie, to keep you signed in, remember your language and theme, and keep your bag and saved items on this device. To date we use no audience measurement tool and no advertising tracker.</P> },
      { id: 'children', title: 'Minors', body: <P>Trezutil is meant for people aged at least {f(LEGAL.minimumAge, 'minimum age')}.</P> },
      { id: 'changes', title: 'Changes to this page', body: <P>The date of the latest version is at the top of the page; an important change is announced on the site before it applies.</P> },
      { id: 'authority', title: 'Complaints', body: <P>You can also contact: {f(LEGAL.authority, 'competent data protection authority')}.</P> },
    ],
  }
}

/* ------------------------------------------------------------ refunds */

function refunds(lang: LegalLang): LegalText {
  const f = fact(lang)
  if (lang === 'fr') {
    return {
      title: 'Annulations et remboursements',
      intro: 'Comment annuler une commande ou une réservation, et être remboursé.',
      sections: [
        { id: 'scope', title: 'Ce que couvre cette page', body: <P>Les commandes et réservations faites sur Trezutil. Un vendeur peut avoir ses propres conditions ; elles s’ajoutent à cette page sans la contredire.</P> },
        { id: 'orders', title: 'Commandes', body: <UL items={[
          'Tant que la commande n’est pas payée, vous pouvez l’annuler sans frais depuis Mes commandes.',
          'Une fois payée, la commande est confirmée par le vendeur. Pour l’annuler, écrivez-nous en indiquant son numéro.',
          <>Délai pour demander un remboursement : {f(LEGAL.refundWindow, 'délai pour demander un remboursement')}.</>,
        ]} /> },
        { id: 'bookings', title: 'Réservations', body: <P>Vous pouvez annuler une réservation depuis Mes réservations : le créneau est alors libéré. Un éventuel remboursement suit les conditions du vendeur et le délai ci-dessus.</P> },
        { id: 'how', title: 'Comment le remboursement est fait', body: <UL items={[
          'Le remboursement est fait sur le moyen de paiement utilisé, par notre prestataire de paiement.',
          'Il ne peut pas dépasser le montant payé.',
          <>Délai avant que l’argent arrive : {f(LEGAL.refundDelay, 'délai de remboursement')}.</>,
        ]} /> },
        { id: 'problems', title: 'Un problème avec un produit ou un service', body: <>
          <P>Le vendeur est responsable de la conformité de ce qu’il vend. Écrivez-nous en indiquant votre numéro de commande ou de réservation : {f('', 'procédure de réclamation')}.</P>
          {contact(lang)}
        </> },
      ],
    }
  }
  return {
    title: 'Cancellations and refunds',
    intro: 'How to cancel an order or a booking, and get refunded.',
    sections: [
      { id: 'scope', title: 'What this page covers', body: <P>Orders and bookings made on Trezutil. A seller may have its own conditions; they add to this page without contradicting it.</P> },
      { id: 'orders', title: 'Orders', body: <UL items={[
        'While the order is not paid, you can cancel it free of charge from My orders.',
        'Once paid, the order is confirmed by the seller. To cancel it, write to us with its number.',
        <>Time limit to ask for a refund: {f(LEGAL.refundWindow, 'time limit to ask for a refund')}.</>,
      ]} /> },
      { id: 'bookings', title: 'Bookings', body: <P>You can cancel a booking from My bookings: the slot is then released. Any refund follows the seller’s conditions and the time limit above.</P> },
      { id: 'how', title: 'How the refund is made', body: <UL items={[
        'The refund is made to the payment method used, by our payment provider.',
        'It cannot exceed the amount paid.',
        <>Time before the money arrives: {f(LEGAL.refundDelay, 'refund delay')}.</>,
      ]} /> },
      { id: 'problems', title: 'A problem with a product or a service', body: <>
        <P>The seller is responsible for the conformity of what it sells. Write to us with your order or booking number: {f('', 'complaint procedure')}.</P>
        {contact(lang)}
      </> },
    ],
  }
}

/* ------------------------------------------------------------- notice */

function notice(lang: LegalLang): LegalText {
  const f = fact(lang)
  const e = LEGAL.entity
  if (lang === 'fr') {
    return {
      title: 'Mentions légales',
      intro: 'Qui édite et héberge Trezutil, et comment signaler un contenu.',
      sections: [
        { id: 'publisher', title: 'Éditeur', body: <UL items={[
          <>Société : {f(e.name, 'nom de la société')} ({f(e.form, 'forme juridique')})</>,
          <>Adresse : {f(e.address, 'adresse du siège')}</>,
          <>Immatriculation : {f(e.registration, 'numéros d’immatriculation')}</>,
          <>Directeur de la publication : {f(e.director, 'nom du directeur de la publication')}</>,
          <>Contact : {f(e.email, 'adresse e-mail')}{e.phone ? `, ${e.phone}` : ''}</>,
        ]} /> },
        { id: 'hosting', title: 'Hébergement', body: <P>{f(e.hosting, 'hébergeur (nom, adresse, pays)')}</P> },
        { id: 'ip', title: 'Propriété intellectuelle', body: <>
          <P>Le nom Trezutil, son logo et les textes du site appartiennent à {f(e.name, 'nom de la société')}. Les logos de nos partenaires appartiennent à leurs propriétaires et ne servent qu’à désigner ces organisations. Les photos et les textes des offres sont publiés sous la responsabilité de chaque vendeur.</P>
        </> },
        { id: 'report', title: 'Signaler un contenu', body: <>
          <P>Pour signaler une offre ou un contenu illicite ou trompeur, écrivez-nous en indiquant l’adresse de la page.</P>
          {contact(lang)}
        </> },
        { id: 'data', title: 'Vos données', body: <P>Voir la <ActionLink href="/legal/privacy" className="legal-link">politique de confidentialité</ActionLink>.</P> },
      ],
    }
  }
  return {
    title: 'Legal notice',
    intro: 'Who publishes and hosts Trezutil, and how to report content.',
    sections: [
      { id: 'publisher', title: 'Publisher', body: <UL items={[
        <>Company: {f(e.name, 'company name')} ({f(e.form, 'legal form')})</>,
        <>Address: {f(e.address, 'registered address')}</>,
        <>Registration: {f(e.registration, 'registration numbers')}</>,
        <>Director of publication: {f(e.director, 'name of the director of publication')}</>,
        <>Contact: {f(e.email, 'email address')}{e.phone ? `, ${e.phone}` : ''}</>,
      ]} /> },
      { id: 'hosting', title: 'Hosting', body: <P>{f(e.hosting, 'host (name, address, country)')}</P> },
      { id: 'ip', title: 'Intellectual property', body: <P>The Trezutil name, its logo and the site’s texts belong to {f(e.name, 'company name')}. Our partners’ logos belong to their owners and only serve to name those organisations. Photos and texts of offers are published under each seller’s responsibility.</P> },
      { id: 'report', title: 'Report content', body: <>
        <P>To report an unlawful or misleading offer or content, write to us with the address of the page.</P>
        {contact(lang)}
      </> },
      { id: 'data', title: 'Your data', body: <P>See the <ActionLink href="/legal/privacy" className="legal-link">privacy policy</ActionLink>.</P> },
    ],
  }
}

const BUILD: Record<LegalDoc, (lang: LegalLang) => LegalText> = { terms, privacy, refunds, notice }

export function legalText(doc: LegalDoc, lang: LegalLang): LegalText {
  return BUILD[doc](lang)
}
