// The preview behaves like the phone the game is made for: touch screen without a keyboard.
// Games that check for touch (maxTouchPoints, ontouchstart, pointer: coarse) pick their mobile
// hints and controls; the mouse acts as one finger: besides its own pointer events it fires
// touchstart/touchmove/touchend, so games written for touch events react to it as well.
const phoneEmulation = `<script>(function(){
var held=null;
function touch(type,e){try{var t=new Touch({identifier:1,target:held,clientX:e.clientX,clientY:e.clientY,pageX:e.pageX,pageY:e.pageY,screenX:e.screenX,screenY:e.screenY,radiusX:11,radiusY:11,force:1});var on=type==='touchend'?[]:[t];held.dispatchEvent(new TouchEvent(type,{bubbles:true,cancelable:true,composed:true,touches:on,targetTouches:on,changedTouches:[t]}))}catch(err){}}
window.addEventListener('mousedown',function(e){if(e.button!==0)return;held=e.target;touch('touchstart',e)},true);
window.addEventListener('mousemove',function(e){if(held)touch('touchmove',e)},true);
window.addEventListener('mouseup',function(e){if(held){touch('touchend',e);held=null}},true);
document.addEventListener('mouseleave',function(e){if(held){touch('touchend',e);held=null}});
try{Object.defineProperty(navigator,'maxTouchPoints',{get:function(){return 5}})}catch(e){}
window.ontouchstart=null;
var mm=window.matchMedia&&window.matchMedia.bind(window);
var stub=function(q,m){return{matches:m,media:q,onchange:null,addListener:function(){},removeListener:function(){},addEventListener:function(){},removeEventListener:function(){},dispatchEvent:function(){return false}}};
if(mm){window.matchMedia=function(q){if(/pointer\\s*:\\s*coarse|hover\\s*:\\s*none/.test(q))return stub(q,true);if(/pointer\\s*:\\s*fine|hover\\s*:\\s*hover/.test(q))return stub(q,false);return mm(q)}}
['keydown','keyup','keypress'].forEach(function(t){window.addEventListener(t,function(e){e.stopImmediatePropagation();e.preventDefault()},true)});
window.addEventListener('contextmenu',function(e){e.preventDefault()},true);
})();<\/script>`;

const fingerCursor =
  "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='32' height='32'%3E%3Ccircle cx='16' cy='16' r='13' fill='rgba(255,255,255,.35)' stroke='rgba(0,0,0,.45)' stroke-width='2'/%3E%3C/svg%3E\") 16 16, pointer";
// a finger does not select text or open the image menu
const cursorStyle = `<style>html,body,body *{cursor:${fingerCursor}!important;-webkit-user-select:none;user-select:none;-webkit-touch-callout:none}</style>`;

export function previewHTML(html) {
  const policy = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' blob:; style-src 'unsafe-inline'; img-src data: blob:; media-src data: blob:; font-src data:; connect-src 'none'; form-action 'none'; base-uri 'none'">`;
  const stub = `<script>window.GameSDK={start(){parent.postMessage({source:'studio-preview',type:'start'},'*')},end(result,score){parent.postMessage({source:'studio-preview',type:'end',data:{result,score}},'*')},event(){}};<\/script>`;
  // before the game and GameKit: they must see the phone from their first line
  const head = `${policy}${stub}${phoneEmulation}${cursorStyle}`;
  if (/<head[^>]*>/i.test(html)) {
    return html.replace(/<head[^>]*>/i, (match) => `${match}${head}`);
  }
  return `<!doctype html><head>${head}</head>` + html.replace(/<!doctype[^>]*>/i, '');
}
