import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { LEGAL, type LegalDoc } from '@/lib/legal'

/*
 * The English version of the legal pages: the same documents as legalContent.tsx, section by section. Keep both files
 * in step. A fact the code cannot know is never invented: it shows as a highlighted marker until it is filled in
 * src/lib/legal.ts.
 */

function Missing({ what }: { what: string }) {
  return <mark className="legal-missing">[To be completed: {what}]</mark>
}

function F({ v, what }: { v: string; what: string }) {
  return v ? <>{v}</> : <Missing what={what} />
}

const E = LEGAL.entity
const Company = () => <><F v={E.name} what="company name" />{E.form ? `, ${E.form}` : ''}</>
const Contact = () => <F v={E.email} what="contact email address" />

function Section({ title, children }: { title: string; children: ReactNode }) {
  return <section><h2>{title}</h2>{children}</section>
}

const terms = (
  <>
    <p className="legal-lede">These terms apply to any use of Trezutil (the trezutil.com website, the conversational assistant and exchanges over WhatsApp). By using the service, you accept them.</p>
    <Section title="1. Who operates Trezutil">
      <p>Trezutil is operated by <Company />, whose registered office is at <F v={E.address} what="registered office address" />. Contact: <Contact />.</p>
    </Section>
    <Section title="2. What Trezutil does">
      <p>Trezutil is a business assistant: you say what you want to buy, sell or get done, and Trezutil helps you find suppliers, compare offers, organise an operation in several steps, then place an order, a booking or a payment.</p>
      <p><b>Nothing is ordered, booked or paid without your explicit confirmation.</b> Before any commitment, Trezutil shows you a summary that you must accept; without that confirmation, no action is carried out.</p>
      <p>Trezutil is not the seller of the products and services offered by suppliers: the sale or service contract is made between you and the supplier. <Missing what="exact legal status of Trezutil’s role (platform, intermediary, agent) — to be confirmed by a lawyer" /></p>
    </Section>
    <Section title="3. Your account">
      <p>You can use Trezutil without an account to search and compare. Orders, bookings, payments, business memory and synchronised history require an account.</p>
      <p>You sign in either with your phone number (a one-time code is sent to you on WhatsApp; no password) or with your Google account. Your first sign-in creates your account. You must be at least <F v={LEGAL.minimumAge} what="minimum age" /> to open an account.</p>
      <p>You are responsible for the use of your account and your phone; never share your sign-in code. Trezutil will never ask you for it.</p>
    </Section>
    <Section title="4. The assistant and artificial intelligence">
      <p>To understand your requests and write its answers, Trezutil uses artificial intelligence models provided by third parties (see the privacy policy). The prices, offers, availability and suppliers shown come from Trezutil’s and the suppliers’ data, never from something the model made up: an answer that would state a figure that is not established is discarded.</p>
      <p>The assistant can still get its understanding or wording wrong. <b>Check important information before any business decision.</b></p>
    </Section>
    <Section title="5. Orders, bookings and payments">
      <p>An order or a booking is created only after your confirmation. Card payments are processed by Stripe; Trezutil neither receives nor stores your card number. Cancellations and refunds follow the <Link to="/legal/remboursements">cancellation and refund rules</Link>.</p>
    </Section>
    <Section title="6. Usage credits">
      <p>Some actions may use credits (for example a search, an availability check or writing by the AI). The cost of each paid action and your balance are shown in Settings → Credits and usage. Credits are first reserved, then used only if the action succeeds; otherwise they are released. Personal credits and an organisation’s credits are separate. <Missing what="price, validity and refundability of purchased credits" /></p>
    </Section>
    <Section title="7. What is not allowed">
      <ul>
        <li>using Trezutil for any illegal, fraudulent or misleading activity;</li>
        <li>offering or requesting prohibited goods or services: <F v={LEGAL.prohibitedItems} what="list of prohibited goods and services" />;</li>
        <li>trying to access another user’s account, conversations or data, or to get around the technical limits of the service;</li>
        <li>sending unlawful or hateful content, or content that infringes the rights of others;</li>
        <li>trying to make the assistant act against its rules (for example to make it skip a confirmation).</li>
      </ul>
    </Section>
    <Section title="8. Suppliers">
      <p>Suppliers registered on Trezutil accept the <Link to="/legal/fournisseurs">supplier terms</Link>. A supplier’s registration is not verification: only suppliers marked “verified” have been checked by the Trezutil team, and that check does not guarantee the quality of every service.</p>
    </Section>
    <Section title="9. Liability">
      <p>Trezutil does its best to keep the service working and the information shown accurate, without being able to guarantee permanent availability. The supplier remains responsible for the products and services it sells. <Missing what="scope and limits of Trezutil’s liability — to be drafted by a lawyer" /></p>
    </Section>
    <Section title="10. Suspension and termination">
      <p>You can stop using Trezutil at any time and ask for your account to be deleted (see the privacy policy). Trezutil may suspend an account that breaches these terms; an order already confirmed remains governed by its own rules.</p>
    </Section>
    <Section title="11. Changes, governing law and disputes">
      <p>These terms may change; the date of the last update is shown at the top of the page and you will be told about important changes. Governing law and competent courts: <F v={E.jurisdiction} what="governing law and competent courts" />. <Missing what="complaint and mediation procedure" /></p>
    </Section>
  </>
)

const privacy = (
  <>
    <p className="legal-lede">This policy explains what data Trezutil processes, why, who it is shared with, how long it is kept and what your rights are.</p>
    <Section title="1. Data controller">
      <p><Company />, <F v={E.address} what="registered office address" />. For any question about your data: <Contact />.</p>
    </Section>
    <Section title="2. The data we process">
      <ul>
        <li><b>Account</b>: phone number (if you sign in through WhatsApp), display name, email address (if you enter it or Google has verified it), account roles and status. With Google: your Google account identifier, your name and your verified email address. Sign-in codes are only kept as a hash, for a few minutes.</li>
        <li><b>Conversations</b>: your messages, Trezutil’s answers, the offers and results shown, the title you give a conversation and whether it is archived.</li>
        <li><b>Business memory</b>: only what you choose to add (preferences, brands, budgets, constraints, favourite suppliers, projects). Trezutil refuses to store card numbers, codes, passwords or bank details in it. You can turn it off or erase it at any time.</li>
        <li><b>Operations, orders, bookings and payments</b>: what you order or book, from whom, the amounts, statuses and payment references. Your card number is entered at Stripe and is never passed to Trezutil.</li>
        <li><b>Credits</b>: your balance and the history of credits reserved, used or released.</li>
        <li><b>Suppliers</b>: for a business that registers, its details (registered name, contact details, legal identifiers, sectors, capabilities, service areas) and the documents sent (RCCM, national identification, NIF, manager’s identity document).</li>
        <li><b>Technical data</b>: IP address (to limit abuse), request identifiers and technical logs. Logs contain no password, token or sign-in code.</li>
        <li><b>Voice</b>: dictation and voice mode use your browser’s speech recognition; the recognised text is sent as a written message. Trezutil does not record your voice.</li>
      </ul>
    </Section>
    <Section title="3. Why">
      <ul>
        <li>to provide the service: sign you in, understand your requests, search and compare, carry out the actions you confirm, synchronise your history;</li>
        <li>to secure the service: prevent fraud, limit abusive attempts, investigate an incident;</li>
        <li>to verify the suppliers who register;</li>
        <li>to meet our legal obligations (accounting, complaints).</li>
      </ul>
      <p>Trezutil does not sell your data and does not use it for advertising. <Missing what="legal basis of each processing activity (contract, legitimate interest, legal obligation, consent) — to be validated by a lawyer" /></p>
    </Section>
    <Section title="4. Who it is shared with">
      <ul>
        <li><b>Anthropic</b> (Claude models, United States): the text of your messages, the conversation context and, if you turned it on, your business memory, to understand your request and write the answer. When no AI provider is available, Trezutil works with a rule-based method, without sending your messages.</li>
        <li><b>Google</b>: if you use “Continue with Google”, Google authenticates you and sends us your identifier, your name and your email address. If the Gemini model is enabled, it may receive the same data as described for Anthropic.</li>
        <li><b>Meta (WhatsApp)</b>: your phone number and the messages exchanged with Trezutil on WhatsApp, and sign-in codes.</li>
        <li><b>Stripe</b>: the information needed for a card payment.</li>
        <li><b>The suppliers</b> you order or book from: the information needed to fulfil your order (name, contact, order details).</li>
        <li><b>Our hosting provider</b>: {LEGAL.hosting}.</li>
      </ul>
      <p>Some of these providers process data outside the Democratic Republic of the Congo. <Missing what="countries concerned and safeguards for these transfers — to be validated by a lawyer" /></p>
    </Section>
    <Section title="5. How long">
      <p><F v={LEGAL.retention} what="retention periods for accounts, conversations, orders, supplier documents and logs" /></p>
      <p>You can delete a conversation yourself (it is then erased from our servers, with its messages) and your business memory. Related orders and bookings are kept for their own obligations.</p>
    </Section>
    <Section title="6. Security">
      <p>Exchanges are encrypted (HTTPS). Sign-in codes are stored as a hash and can only be tried a limited number of times. Each account can only access its own data; supplier documents are only visible to the business’s managers and the Trezutil team. Databases are backed up before every update of the service.</p>
    </Section>
    <Section title="7. Your rights">
      <p>You can ask to access, correct or delete your data, object to a processing activity or ask for its portability by writing to <Contact />. You can correct your name and email in Settings, and delete your conversations and business memory directly in the app. Full deletion of an account is currently done on written request. You can also contact the data protection authority: <F v={LEGAL.authority} what="competent data protection authority" />.</p>
    </Section>
    <Section title="8. Minors">
      <p>Trezutil is not intended for people under <F v={LEGAL.minimumAge} what="minimum age" />.</p>
    </Section>
    <Section title="9. Cookies">
      <p>See <Link to="/legal/cookies">Cookies and Local Storage</Link>.</p>
    </Section>
  </>
)

const cookies = (
  <>
    <p className="legal-lede">Trezutil uses <b>no advertising cookies and no audience measurement tools</b>. That is why no consent banner is shown.</p>
    <Section title="What Trezutil stores on your device">
      <ul>
        <li>your session (sign-in tokens), to keep you signed in;</li>
        <li>a copy of your conversations and business memory, to show them immediately and keep a change made offline until the next synchronisation;</li>
        <li>your preferences: language, theme, chosen profile (personal or business), notifications.</li>
      </ul>
      <p>This data stays in your browser’s local storage. Signing out erases the session; you can erase everything from your browser settings.</p>
    </Section>
    <Section title="Third-party services">
      <ul>
        <li><b>Google</b>: if you use “Continue with Google”, the button is provided by Google, which may set its own cookies to authenticate you.</li>
        <li><b>Stripe</b>: during a card payment, Stripe may set cookies needed for payment security and fraud prevention.</li>
      </ul>
      <p>If an audience measurement tool were ever added, your consent would be asked first.</p>
    </Section>
  </>
)

const refunds = (
  <>
    <p className="legal-lede">These rules apply to orders and bookings made through Trezutil.</p>
    <Section title="Cancelling">
      <p>You can cancel an order or a booking from the app (Purchases and bookings) as long as its status allows it. A cancellation is never made on your behalf without your request.</p>
    </Section>
    <Section title="Getting a refund">
      <p>A card payment that is cancelled or refused by the supplier is refunded to the payment method used, through Stripe. A refund can never exceed what was paid. Deadline to ask for a refund: <F v={LEGAL.refundWindow} what="deadline to ask for a refund" />. Time before you receive the money: <F v={LEGAL.refundDelay} what="refund delay" />.</p>
    </Section>
    <Section title="Dispute with a supplier">
      <p>The supplier is responsible for the product or service delivered. If there is a problem, first contact the supplier from your order, then the Trezutil team at <Contact /> if no solution is found. <Missing what="complaint procedure, conditions for non-returnable products and applicable right of withdrawal" /></p>
    </Section>
  </>
)

const suppliers = (
  <>
    <p className="legal-lede">These terms apply to businesses and people who register as suppliers on Trezutil, in addition to the terms of use.</p>
    <Section title="1. Registration and verification">
      <p>Registration is free and done online: the person who registers the business becomes its owner on Trezutil. You agree to provide accurate information and authentic documents and to keep them up to date.</p>
      <p>Three separate stages: <b>registered</b> (application in progress), <b>documents submitted</b> (pending), <b>verified</b>. Only the Trezutil team decides on verification, after reviewing the documents; it may ask for more information or reject an application, saying why. Registration never means verification.</p>
    </Section>
    <Section title="2. Your commitments">
      <ul>
        <li>offer real prices, availability and terms, and honour the offers and orders you accept;</li>
        <li>offer no prohibited (<F v={LEGAL.prohibitedItems} what="list of prohibited goods and services" />), counterfeit or dangerous product or service;</li>
        <li>comply with the laws that apply to your activity (licences, tax, consumer protection);</li>
        <li>use buyers’ data only to process their orders and bookings, never for any other purpose or to resell it.</li>
      </ul>
    </Section>
    <Section title="3. Requests, negotiations and orders">
      <p>Trezutil may pass buyers’ requests on to you and, when the buyer allows it, negotiate on their behalf within the limits they set. A proposal only binds the buyer after their explicit confirmation. Your answers are kept in the negotiation history.</p>
    </Section>
    <Section title="4. Payments and fees">
      <p>Trezutil fees: <F v={LEGAL.supplierFees} what="commission or subscription charged to suppliers" />. <Missing what="supplier payout terms (payment account, payout delays, KYC)" /></p>
    </Section>
    <Section title="5. Documents and data">
      <p>The documents you send are only visible to your business’s managers and the Trezutil team, and are used only for verification. See the <Link to="/legal/confidentialite">privacy policy</Link>.</p>
    </Section>
    <Section title="6. Suspension">
      <p>Trezutil may suspend or remove a supplier who provides false information, does not keep their commitments or is the subject of repeated complaints, after informing them except in an emergency.</p>
    </Section>
  </>
)

const notice = (
  <>
    <Section title="Publisher">
      <p><Company /><br /><F v={E.address} what="registered office address" /><br /><F v={E.registration} what="RCCM, national identification, NIF" /><br />Contact: <Contact /> · <F v={E.phone} what="phone" /></p>
    </Section>
    <Section title="Publication director">
      <p><F v={E.director} what="name of the publication director" /></p>
    </Section>
    <Section title="Hosting">
      <p>{LEGAL.hosting}</p>
    </Section>
    <Section title="Intellectual property">
      <p>The Trezutil brand, logo and interface belong to <Company />. The brands and logos of partners and suppliers belong to their owners.</p>
    </Section>
  </>
)

export const LEGAL_CONTENT_EN: Record<LegalDoc, ReactNode> = {
  conditions: terms,
  confidentialite: privacy,
  cookies,
  remboursements: refunds,
  fournisseurs: suppliers,
  mentions: notice,
}
