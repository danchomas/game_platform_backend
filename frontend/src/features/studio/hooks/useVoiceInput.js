import { useEffect, useRef, useState } from 'react';

export function useVoiceInput({ onTranscript, notify }) {
  const [recording, setRecording] = useState(false);
  const recognition = useRef(null);
  useEffect(() => () => recognition.current?.abort(), []);
  function toggleRecording() {
    const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!SpeechRecognition) {
      notify('В этом браузере голосовой ввод недоступен. Введите запрос текстом.');
      return;
    }
    if (recording) {
      recognition.current?.stop();
      setRecording(false);
      return;
    }
    const instance = new SpeechRecognition();
    instance.lang = 'ru-RU';
    instance.interimResults = false;
    instance.onresult = (event) => onTranscript(event.results[0][0].transcript);
    instance.onerror = () => {
      setRecording(false);
      notify('Микрофон недоступен. Проверьте разрешение браузера.');
    };
    instance.onend = () => setRecording(false);
    recognition.current = instance;
    try {
      instance.start();
      setRecording(true);
    } catch {
      setRecording(false);
      notify('Не удалось включить микрофон.');
    }
  }
  return {
    recording,
    toggleRecording,
  };
}
