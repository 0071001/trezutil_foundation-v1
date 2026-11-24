// Runs before any app module loads: a French browser, so labels computed when a module loads are in French too
// (jsdom reports en-US, which the app would detect as English on a first visit).
Object.defineProperty(window.navigator, 'language', { value: 'fr-FR', configurable: true })
