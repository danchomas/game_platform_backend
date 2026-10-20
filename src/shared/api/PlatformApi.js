/** Server contract. Delegates transport to an injected HTTP client through composition. */
export class PlatformApi {
  #httpClient;
  constructor(httpClient) {
    this.#httpClient = httpClient;
  }
  register(credentials) {
    return this.#httpClient.request('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify(credentials),
      timeout: 15_000,
    });
  }
  login(accessKey) {
    return this.#httpClient.request('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        access_key: accessKey,
      }),
      timeout: 15_000,
    });
  }
  listGames({ collection = 'mine', offset = 0 } = {}) {
    const limit = collection === 'public' ? 100 : 200;
    const path = collection === 'public' ? '/api/gallery' : '/api/games/mine';
    return this.#httpClient.request(`${path}?limit=${limit}&offset=${offset}`);
  }

  // Authenticated export also supports drafts without relying on the public game endpoint.
  getGameHTML(gameId) {
    return this.#httpClient.request(`${this.#gamePath(gameId)}/export`, {
      text: true,
    });
  }
  getMessages(gameId) {
    return this.#httpClient.request(`${this.#gamePath(gameId)}/messages`);
  }
  getVersions(gameId) {
    return this.#httpClient.request(`${this.#gamePath(gameId)}/versions`);
  }
  createGame(prompt) {
    return this.#post('/api/games', {
      prompt,
    });
  }
  iterateGame(gameId, content) {
    return this.#post(`${this.#gamePath(gameId)}/messages`, {
      content,
    });
  }
  restoreVersion(gameId, version) {
    return this.#post(`${this.#gamePath(gameId)}/versions/${encodeURIComponent(version)}/restore`);
  }
  setPublished(gameId, isPublished) {
    return this.#post(`${this.#gamePath(gameId)}/${isPublished ? 'publish' : 'unpublish'}`);
  }
  renameGame(gameId, title) {
    return this.#httpClient.request(this.#gamePath(gameId), {
      method: 'PATCH',
      body: JSON.stringify({
        title,
      }),
    });
  }
  deleteGame(gameId) {
    return this.#httpClient.request(this.#gamePath(gameId), {
      method: 'DELETE',
    });
  }
  getAnalytics({ days, gameId = 'all', scope = 'mine' }) {
    const path =
      gameId === 'all'
        ? `/api/analytics/overview?days=${days}&scope=${scope}`
        : `/api/analytics/games/${encodeURIComponent(gameId)}?days=${days}`;
    return this.#httpClient.request(path);
  }
  #gamePath(gameId) {
    return `/api/games/${encodeURIComponent(gameId)}`;
  }
  #post(path, body) {
    return this.#httpClient.request(path, {
      method: 'POST',
      ...(body
        ? {
            body: JSON.stringify(body),
          }
        : {}),
    });
  }
}
