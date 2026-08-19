// Tombstone service worker.
//
// This app no longer uses a service worker (see rollup.config.js). Earlier
// builds registered a Workbox precaching SW that kept serving stale JS/CSS
// across rebuilds. Browsers that still have that SW registered will re-fetch
// this file on their next navigation; because it differs from the old SW, it
// installs, activates, unregisters itself, purges every cache it created, and
// reloads open tabs so they pick up fresh assets. New visitors register nothing.
self.addEventListener('install', () => self.skipWaiting());

self.addEventListener('activate', event => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(keys.map(key => caches.delete(key)));
      await self.registration.unregister();
      const clients = await self.clients.matchAll({ type: 'window' });
      clients.forEach(client => client.navigate(client.url));
    })()
  );
});
