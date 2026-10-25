import React from 'react';

export class ErrorBoundary extends React.Component {
  state = {
    error: false,
  };
  static getDerivedStateFromError() {
    return {
      error: true,
    };
  }
  render() {
    if (this.state.error) {
      return (
        <div className="fatal-error">
          <h1>Студии нужен перезапуск</h1>
          <p>Данные сохранены в браузере. Обновите страницу, чтобы продолжить.</p>
          <button className="button yellow" onClick={() => window.location.reload()}>
            Обновить страницу
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
