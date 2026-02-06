var CACHE_NAME = 'gamwich-v1';
var CDN_ASSETS = [
    'https://cdn.jsdelivr.net/npm/daisyui@4.12.23/dist/full.min.css',
    'https://cdn.tailwindcss.com',
    'https://unpkg.com/htmx.org@2.0.4',
    'https://cdn.jsdelivr.net/npm/alpinejs@3.14.8/dist/cdn.min.js'
];

self.addEventListener('install', function(event) {
    event.waitUntil(
        caches.open(CACHE_NAME).then(function(cache) {
            return cache.addAll(CDN_ASSETS);
        })
    );
    self.skipWaiting();
});

self.addEventListener('activate', function(event) {
    event.waitUntil(
        caches.keys().then(function(names) {
            return Promise.all(
                names.filter(function(name) { return name !== CACHE_NAME; })
                    .map(function(name) { return caches.delete(name); })
            );
        })
    );
    self.clients.claim();
});

self.addEventListener('fetch', function(event) {
    if (event.request.method !== 'GET') return;

    // Skip WebSocket upgrade requests
    if (event.request.headers.get('Upgrade') === 'websocket') return;

    event.respondWith(
        fetch(event.request).then(function(response) {
            if (response.ok) {
                var clone = response.clone();
                caches.open(CACHE_NAME).then(function(cache) {
                    cache.put(event.request, clone);
                });
            }
            return response;
        }).catch(function() {
            return caches.match(event.request);
        })
    );
});
