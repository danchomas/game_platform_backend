export function GameArt({ kind = 'snake', color, className = '' }) {
  return (
    <div
      className={`game-art ${kind} ${className}`}
      style={{
        backgroundColor: color,
      }}
      aria-hidden="true"
    >
      <div className="art-grain" />
      {kind === 'snake' ? (
        <svg viewBox="0 0 320 210" fill="none">
          <path
            d="M48 156H203Q245 156 245 113V91Q245 52 205 52H137Q103 52 103 87Q103 120 138 120H186"
            stroke="#43553a"
            strokeWidth="34"
            strokeLinecap="round"
          />
          <circle cx="177" cy="113" r="4" fill="#f5efc3" />
          <path d="M207 121h16m-5-6 6 6-6 6" stroke="#9e8633" strokeWidth="3" />
          <circle cx="279" cy="99" r="17" fill="#cb9340" />
          <path d="m279 82 6-10" stroke="#52613b" strokeWidth="3" />
          <path d="m50 54 6-14m-13 6 19 1" stroke="#6f7c4855" strokeWidth="2" />
        </svg>
      ) : kind === 'memory' ? (
        <svg viewBox="0 0 320 210">
          <g transform="rotate(-13 120 110)">
            <rect x="60" y="33" width="111" height="140" rx="23" fill="#829a72" />
            <path d="m116 67 8 25 26 0-21 16 8 25-21-15-21 15 8-25-21-16h26z" fill="#e9edcd" />
          </g>
          <g transform="rotate(14 210 120)">
            <rect x="154" y="50" width="111" height="140" rx="23" fill="#f8f7e7" />
            <path d="m210 86 8 25 26 0-21 16 8 25-21-15-21 15 8-25-21-16h26z" fill="#829a72" />
          </g>
        </svg>
      ) : kind === 'catch' ? (
        <svg viewBox="0 0 320 210">
          <circle cx="154" cy="116" r="55" fill="#a596b5" />
          <circle cx="224" cy="56" r="25" fill="#fff4c7" />
          <circle cx="77" cy="48" r="13" fill="#c4b3d6" />
          <path d="M122 116q32 35 64 0" stroke="#e7dff3" strokeWidth="5" fill="none" />
          <circle cx="135" cy="101" r="5" fill="#e7dff3" />
          <circle cx="172" cy="101" r="5" fill="#e7dff3" />
          <path d="m245 143 6 16 17 1-13 11 4 17-14-10-15 10 5-17-14-11 18-1z" fill="#a596b5" />
        </svg>
      ) : (
        <svg viewBox="0 0 320 210" fill="none">
          <circle cx="160" cy="107" r="79" stroke="#d5986166" />
          <circle cx="160" cy="107" r="60" stroke="#d59861" />
          <circle cx="160" cy="107" r="41" fill="#bd7e4c" />
          <path d="m168 78-25 33h18l-9 24 28-35h-19z" fill="#f9e6c9" />
          <circle cx="226" cy="62" r="9" fill="#edc08c" />
          <circle cx="88" cy="138" r="6" fill="#bd7e4c" />
        </svg>
      )}
      <span className="art-corner">✳</span>
      <span className="art-edition">PLAY A LITTLE.</span>
    </div>
  );
}
