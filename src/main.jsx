import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './styles.css';
class ErrorBoundary extends React.Component {
  state={error:false};
  static getDerivedStateFromError(){return {error:true};}
  render(){if(this.state.error)return <div className="fatal-error"><h1>Студии нужен перезапуск</h1><p>Данные сохранены в браузере. Обновите страницу, чтобы продолжить.</p><button className="button yellow" onClick={()=>window.location.reload()}>Обновить страницу</button></div>;return this.props.children;}
}
createRoot(document.getElementById('root')).render(<React.StrictMode><ErrorBoundary><App/></ErrorBoundary></React.StrictMode>);
