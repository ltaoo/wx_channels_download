// Browser shim adapted from findrss-scripts douyin_sdk_environment.js.
// shortcut: Only the browser APIs used by the signing SDK are emulated; extend when the SDK changes.
var __sdk_phase = 'environment';

// Goja's TypedArray.from requires an object; the SDK also passes binary strings.
var typed_array_from = Uint8Array.from;
Uint8Array.from = function(value, fn, receiver) {
  return typed_array_from.call(this, typeof value === 'string' ? Object(value) : value, fn, receiver);
};

var request = JSON.parse(__sdk_request);
var window = globalThis, self = globalThis;
function noop() {}
var console = { log: noop, warn: noop, error: noop };
var setTimeout = noop, setInterval = noop, clearTimeout = noop, clearInterval = noop;
var addEventListener = noop, removeEventListener = noop, requestAnimationFrame = noop;
var performance = { now: function(){ return 1; }, timing: { navigationStart: Date.now() } };
var localStorage = { getItem: function(){return null;}, setItem: noop, removeItem: noop };
var sessionStorage = localStorage, history = { length: 1 };
function form_encode(value) { return encodeURIComponent(value).replace(/[!'()~]/g,function(c){return '%'+c.charCodeAt(0).toString(16).toUpperCase();}).replace(/%20/g,'+'); }
function URLSearchParams(value) {
  this.pairs = String(value || '').replace(/^\?/,'').split('&').filter(Boolean).map(function(pair){var i=pair.indexOf('=');return [decodeURIComponent((i<0?pair:pair.slice(0,i)).replace(/\+/g,' ')),decodeURIComponent((i<0?'':pair.slice(i+1)).replace(/\+/g,' '))];});
}
URLSearchParams.prototype.get = function(k){var p=this.pairs.find(function(p){return p[0]===k;});return p?p[1]:null;};
URLSearchParams.prototype.has = function(k){return this.pairs.some(function(p){return p[0]===k;});};
URLSearchParams.prototype.delete = function(k){this.pairs=this.pairs.filter(function(p){return p[0]!==k;});};
URLSearchParams.prototype.append = function(k,v){this.pairs.push([String(k),String(v)]);};
URLSearchParams.prototype.set = function(k,v){this.delete(k);this.append(k,v);};
URLSearchParams.prototype.toString = function(){return this.pairs.map(function(p){return form_encode(p[0])+'='+form_encode(p[1]);}).join('&');};
URLSearchParams.prototype.forEach = function(fn){this.pairs.forEach(function(p){fn(p[1],p[0]);});};
function URL(value,base){
 value=String(value);
 if(!/^https?:\/\//.test(value)){var root=String(base||request.url).match(/^https?:\/\/[^/]+/)[0];value=root+(value.charAt(0)==='/'?'':'/')+value;}
 var p=value.match(/^(https?:)\/\/([^/?#]+)([^?#]*)(\?[^#]*)?(#.*)?$/);
 if(!p)throw new Error('SDK URL invalid');
 this.protocol=p[1];this.host=p[2];this.hostname=p[2].split(':')[0];this.pathname=p[3]||'/';this.hash=p[5]||'';this.origin=p[1]+'//'+p[2];this.searchParams=new URLSearchParams(p[4]||'');
}
Object.defineProperty(URL.prototype,'search',{get:function(){var q=this.searchParams.toString();return q?'?'+q:'';},set:function(v){this.searchParams=new URLSearchParams(v);}});
Object.defineProperty(URL.prototype,'href',{get:function(){return this.origin+this.pathname+this.search+this.hash;}});
URL.prototype.toString = function(){return this.href;};
var location = new URL('https://www.douyin.com/');
var navigator = { userAgent: request.user_agent, platform: 'MacIntel', language: 'zh-CN', languages: ['zh-CN','zh'], vendorSubs:{ink:Date.now()-1}, cookieEnabled: true, hardwareConcurrency:10, deviceMemory:32 };
var screen = {width:1512,height:982,availWidth:1512,availHeight:982,colorDepth:24,pixelDepth:24};
var innerWidth=1512,innerHeight=857,outerWidth=1512,outerHeight=982,screenX=0,screenY=0,pageYOffset=0;
var document = { referrer: 'https://www.douyin.com/', readyState:'complete', currentScript:{getAttribute:function(name){return name==='project-id'?'34':'';}}, querySelector:function(){return this.currentScript;},createElement:function(){return [];},createEvent:noop,documentElement:{},body:[],addEventListener:noop,removeEventListener:noop,getElementsByTagName:function(){return [];} };
// SDK 写入单个 Cookie 时必须保留 provider 注入的 UIFID。
var sdk_cookies = new Map(String(request.cookie).split(/;\s*/).filter(Boolean).map(function(pair){var i=pair.indexOf('=');return [pair.slice(0,i),pair.slice(i+1)];}));
Object.defineProperty(document,'cookie',{
 get:function(){return Array.from(sdk_cookies,function(pair){return pair.join('=');}).join('; ');},
 set:function(value){var parts=String(value).split(';'),pair=parts[0],i=pair.indexOf('=');if(i<1)return;var name=pair.slice(0,i).trim();var expired=parts.slice(1).some(function(attr){var p=attr.trim().split('='),key=p.shift().toLowerCase(),v=p.join('=');return key==='max-age'?Number(v)<=0:key==='expires'&&Date.parse(v)<=Date.now();});if(expired)sdk_cookies.delete(name);else sdk_cookies.set(name,pair.slice(i+1));}
});
var XMLHttpRequest = function(){};
// 只捕获官方 SDK 改写后的请求，不在 VM 内联网。
var signed_request_url;
XMLHttpRequest.prototype.open=function(method,url){this.url=String(url);this.headers={};};
XMLHttpRequest.prototype.send=function(){
  var url=new URL(this.url),target=new URL(request.url);
  if(url.origin===target.origin && url.pathname===target.pathname)signed_request_url=url.href;
};
XMLHttpRequest.prototype.setRequestHeader=function(name,value){this.headers[name]=value;};
var fetch=noop;
var onwheelx={_Ax:'0X21'};
var gfdatav1={envName:'prod'};
var _secsdk_uifid=sdk_cookies.get('UIFID')||sdk_cookies.get('UIFID_TEMP')||'';
var SSR_RENDER_DATA={app:{envService:'prod',odin:{user_id:_secsdk_uifid}}};
function atob(value){var alphabet='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/',bits=0,count=0,out='';String(value).replace(/=+$/,'').split('').forEach(function(c){var n=alphabet.indexOf(c);if(n<0)return;bits=(bits<<6)|n;count+=6;if(count>=8){count-=8;out+=String.fromCharCode((bits>>>count)&255);}});return out;}
function btoa(value){var alphabet='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/',out='';for(var i=0;i<value.length;i+=3){var n=(value.charCodeAt(i)<<16)|((value.charCodeAt(i+1)||0)<<8)|(value.charCodeAt(i+2)||0);out+=alphabet[(n>>>18)&63]+alphabet[(n>>>12)&63]+(i+1<value.length?alphabet[(n>>>6)&63]:'=')+(i+2<value.length?alphabet[n&63]:'=');}return out;}

__sdk_phase = 'bdms_load';
eval(__bdms_source);

var target = new URL(request.url);
target.searchParams.delete('a_bogus');
var signing_time = Math.floor(Date.now()/1000);
target.searchParams.set('timestamp',String(signing_time));
Date.now = function(){return signing_time*1000;};
var verify_fp = sdk_cookies.get('s_v_web_id');
if(verify_fp){target.searchParams.set('verifyFp',verify_fp);target.searchParams.set('fp',verify_fp);}
__sdk_phase = 'bdms_sign';
bdms.init({aid:6383,pageId:6241,boe:false,paths:['^/aweme/v1/'],ddrt:8.5,ic:8.5});
var xhr=new XMLHttpRequest();
xhr.open('GET',target.href,true);
xhr.send();
if(!signed_request_url || !new URL(signed_request_url).searchParams.get('a_bogus'))throw new Error('Douyin BDMS did not sign the request');

__sdk_phase = 'security_load';
eval(__security_source);

__sdk_phase = 'security_sign';
var secured = use('webSignUrl')(signed_request_url);
if(!secured.headers['x-secsdk-web-signature'])throw new Error('Douyin security SDK did not generate signature');
JSON.stringify(secured);
