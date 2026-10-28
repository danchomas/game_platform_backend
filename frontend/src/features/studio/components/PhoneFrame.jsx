import { useLayoutEffect, useRef, useState } from 'react';

// A typical modern phone screen in CSS pixels (iPhone 14/15, most Android flagships ~ the same ratio).
export const PHONE = { width: 390, height: 844 };
const BEZEL = 12;
const MAX_SCALE = 1.25;

export function phoneScreen(orientation) {
  return orientation === 'landscape' ? { width: PHONE.height, height: PHONE.width } : { ...PHONE };
}

// The game gets the phone's real viewport (390×844) and the frame is scaled to fit the panel,
// so layout, font sizes and gestures match what the player sees on the device.
export function PhoneFrame({ orientation, children }) {
  const stage = useRef(null);
  const [scale, setScale] = useState(0);
  const screen = phoneScreen(orientation);

  useLayoutEffect(() => {
    const element = stage.current;
    if (!element) {
      return;
    }
    const fit = () => {
      const { width, height } = element.getBoundingClientRect();
      const next = Math.min(
        (width - BEZEL * 2) / screen.width,
        (height - BEZEL * 2) / screen.height,
        MAX_SCALE,
      );
      setScale(Math.max(0.2, next));
    };
    fit();
    const observer = new ResizeObserver(fit);
    observer.observe(element);
    return () => observer.disconnect();
  }, [screen.width, screen.height]);

  return (
    <div className="phone-stage" ref={stage}>
      <div
        className={`phone-frame ${orientation}`}
        style={{
          width: screen.width * scale + BEZEL * 2,
          height: screen.height * scale + BEZEL * 2,
          visibility: scale ? 'visible' : 'hidden',
        }}
      >
        <div
          className="phone-screen"
          style={{ width: screen.width * scale, height: screen.height * scale }}
        >
          <div
            className="phone-viewport"
            style={{
              width: screen.width,
              height: screen.height,
              transform: `scale(${scale})`,
            }}
          >
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}
