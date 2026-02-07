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

// Push notification handler
self.addEventListener('push', function(event) {
    var data = { title: 'Gamwich', body: 'You have a new notification' };
    if (event.data) {
        try {
            data = event.data.json();
        } catch (e) {
            data.body = event.data.text();
        }
    }

    var options = {
        body: data.body,
        icon: '/static/icon-192.png',
        badge: '/static/icon-192.png',
        tag: data.tag || 'gamwich-notification',
        data: { url: data.url || '/' },
        renotify: true
    };

    event.waitUntil(
        self.registration.showNotification(data.title, options)
    );
});

// Notification click handler
self.addEventListener('notificationclick', function(event) {
    event.notification.close();

    var url = event.notification.data && event.notification.data.url
        ? event.notification.data.url
        : '/';

    event.waitUntil(
        clients.matchAll({ type: 'window', includeUncontrolled: true }).then(function(clientList) {
            for (var i = 0; i < clientList.length; i++) {
                var client = clientList[i];
                if (client.url.indexOf(url) !== -1 && 'focus' in client) {
                    return client.focus();
                }
            }
            if (clients.openWindow) {
                return clients.openWindow(url);
            }
        })
    );
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
