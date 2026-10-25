const DEFAULT_TIMEOUT_MS = 240_000;

/** Handles transport only. PlatformApi owns server routes; hooks own product scenarios. */

/** Handles transport only. PlatformApi owns server routes; hooks own product scenarios. */

/** Handles transport only. PlatformApi owns server routes; hooks own product scenarios. */

/** Handles transport only. PlatformApi owns server routes; hooks own product scenarios. */
export class HttpClient {
  #baseUrl;
  #accessToken;
  #fetch;
  constructor({
    baseUrl = '',
    accessToken = '',
    fetchImplementation = (...args) => globalThis.fetch(...args),
  } = {}) {
    this.#baseUrl = baseUrl.replace(/\/$/, '');
    this.#accessToken = accessToken;
    this.#fetch = fetchImplementation;
  }
  async request(path, options = {}) {
    const { timeout = DEFAULT_TIMEOUT_MS, text = false, signal, ...requestOptions } = options;
    const controller = new AbortController();
    const abortRequest = () => controller.abort();
    const timeoutId = setTimeout(abortRequest, timeout);
    if (signal?.aborted) {
      controller.abort();
    } else {
      signal?.addEventListener('abort', abortRequest, {
        once: true,
      });
    }
    try {
      const response = await this.#fetch(`${this.#baseUrl}${path}`, {
        ...requestOptions,
        signal: controller.signal,
        headers: this.#buildHeaders(requestOptions),
      });
      return await this.#readResponse(response, text);
    } catch (error) {
      if (error.name === 'AbortError') {
        if (signal?.aborted) {
          throw error;
        }
        throw new Error(
          'Сервер не успел ответить. Обновите список игр перед повторной генерацией.',
        );
      }
      if (error instanceof TypeError) {
        throw new Error(
          'Нет соединения с сервером. Проверьте, что бэкенд запущен и адрес указан верно.',
        );
      }
      throw error;
    } finally {
      clearTimeout(timeoutId);
      signal?.removeEventListener('abort', abortRequest);
    }
  }
  #buildHeaders({ body, headers }) {
    return {
      ...(body
        ? {
            'Content-Type': 'application/json',
          }
        : {}),
      ...(this.#accessToken
        ? {
            Authorization: `Bearer ${this.#accessToken}`,
          }
        : {}),
      ...headers,
    };
  }
  async #readResponse(response, isTextResponse) {
    if (!response.ok) {
      const errorBody = await response.json().catch(() => null);
      throw new Error(readErrorMessage(errorBody) || `Сервер ответил с ошибкой ${response.status}`);
    }
    if (response.status === 204) {
      return null;
    }
    if (isTextResponse) {
      return response.text();
    }
    const contentType = response.headers.get('content-type') || '';
    if (!contentType.includes('application/json')) {
      throw new Error('API недоступен. Проверьте адрес сервера в настройках подключения.');
    }
    return response.json();
  }
}

// Ошибка приходит как {"error": {"code", "message", "field"}}. Строковая
// форма поддерживается для совместимости со старыми сборками бэкенда.
function readErrorMessage(body) {
  const error = body?.error;
  if (!error) {
    return null;
  }
  return typeof error === 'string' ? error : error.message || null;
}
