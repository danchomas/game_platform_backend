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
  // Returns { game, plan, reply }: the server drafts a plan that must be approved before the build.
  createGame(prompt) {
    return this.#post('/api/games', {
      prompt,
    });
  }
  revisePlan(gameId, content) {
    return this.#post(`${this.#gamePath(gameId)}/plan`, {
      content,
    });
  }
  // Starts the background build; an edited plan replaces the stored one.
  approvePlan(gameId, spec) {
    return this.#post(`${this.#gamePath(gameId)}/approve`, spec ? { spec } : undefined);
  }
  getStatus(gameId) {
    return this.#httpClient.request(`${this.#gamePath(gameId)}/status`, {
      timeout: 15_000,
    });
  }
  // Starts a background edit; progress is read through getStatus.
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
  async getAnalytics({ days, gameId = 'all', scope = 'mine' }) {
    const path =
      gameId === 'all'
        ? `/api/analytics/overview?days=${days}&scope=${scope}`
        : `/api/analytics/games/${encodeURIComponent(gameId)}?days=${days}`;
    const data = await this.#httpClient.request(path);
    // Go encodes nil slices as null; views always receive collections they can render.
    return {
      ...data,
      summary: data?.summary ?? {},
      daily: data?.daily ?? [],
      games: data?.games ?? [],
      custom_events: data?.custom_events ?? [],
      session_duration: data?.session_duration ?? [],
    };
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
