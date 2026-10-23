/**
 * The map page: MapLibre GL JS with a vector style, drawing markers as HTML
 * elements and the route as a line layer. The app pushes the whole MapState via
 * `window.__update(state)`; the page reports `ready` and user pans (`centre`).
 * Camera moves are eased (no bounce): fitBounds / flyTo / easeTo with padding.
 */

export const DEFAULT_STYLE = "https://tiles.openfreemap.org/styles/positron";
/** MapLibre is loaded from the first CDN that answers; the page shows a notice if none does. */
const MAPLIBRE_CDNS = [
  "https://cdnjs.cloudflare.com/ajax/libs/maplibre-gl/4.7.1/maplibre-gl",
  "https://cdn.jsdelivr.net/npm/maplibre-gl@4.7.1/dist/maplibre-gl",
  "https://unpkg.com/maplibre-gl@4.7.1/dist/maplibre-gl",
];

export function mapHtml(styleUrl: string, centre: { lat: number; lng: number }, zoom: number): string {
  return `<!doctype html><html><head>
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
<style>
  html,body,#map{margin:0;padding:0;height:100%;width:100%;overflow:hidden;background:#eef0f2}
  .maplibregl-ctrl-attrib{font-size:9px;opacity:.7}
  .pin{display:flex;flex-direction:column;align-items:center;gap:4px;font-family:-apple-system,Roboto,sans-serif;pointer-events:none}
  .pill{border-radius:10px;padding:6px 12px;text-align:center;line-height:1.15;white-space:nowrap;box-shadow:0 2px 6px rgba(0,0,0,.15)}
  .pill .l{font-size:12px}.pill .v{font-size:17px;font-weight:700}
  .pill.one .l{font-size:13px;font-weight:700}
  .dot{width:12px;height:12px;border-radius:50%;border-width:5px;border-style:solid}
  .car{width:26px;height:26px;border-radius:50%;border:2px solid #fff;display:flex;align-items:center;justify-content:center;box-shadow:0 1px 4px rgba(0,0,0,.25)}
  .car svg{width:15px;height:15px}
  .self{width:16px;height:16px;border-radius:50%;border:3px solid #fff;box-shadow:0 0 0 8px rgba(66,133,244,.18)}
  .stop{width:12px;height:12px;border-radius:50%;border-width:3px;border-style:solid;box-shadow:0 1px 3px rgba(0,0,0,.25)}
  #notice{position:absolute;inset:0;display:none;align-items:center;justify-content:center;font-family:-apple-system,Roboto,sans-serif;font-size:14px;color:#6b7280;text-align:center;padding:24px}
</style></head><body><div id="map"></div><div id="notice">The map could not load. Check the connection and try again.</div><script>
(function(){
  var CDNS=${JSON.stringify(MAPLIBRE_CDNS)};
  function loadLib(i){
    if(i>=CDNS.length){document.getElementById("notice").style.display="flex";return;}
    var l=document.createElement("link");l.rel="stylesheet";l.href=CDNS[i]+".css";document.head.appendChild(l);
    var sc=document.createElement("script");sc.src=CDNS[i]+".js";sc.onload=start;sc.onerror=function(){loadLib(i+1);};document.head.appendChild(sc);
  }
  function start(){
  var post=function(m){var s=JSON.stringify(m);if(window.ReactNativeWebView)window.ReactNativeWebView.postMessage(s);else if(window.parent)window.parent.postMessage(s,"*");};
  var map=new maplibregl.Map({container:"map",style:${JSON.stringify(styleUrl)},center:[${centre.lng},${centre.lat}],zoom:${zoom},attributionControl:{compact:true},dragRotate:false,pitchWithRotate:false});
  map.touchZoomRotate.disableRotation();
  var markers={},state=null,lastCamera="",loaded=false;
  var CAR='<svg viewBox="0 0 24 24"><path fill="currentColor" d="M5 11l1.5-4.5h11L19 11M17.5 16a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3M6.5 16a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3M18.9 6c-.2-.6-.8-1-1.4-1h-11c-.7 0-1.2.4-1.4 1L3 12v8a1 1 0 0 0 1 1h1a1 1 0 0 0 1-1v-1h12v1a1 1 0 0 0 1 1h1a1 1 0 0 0 1-1v-8z"/></svg>';
  function el(m,c){
    var d=document.createElement("div");
    if(m.kind==="car"){d.className="car";d.style.background=c.primary;d.style.color=c.onPrimary;d.innerHTML=CAR;return d;}
    if(m.kind==="self"){d.className="self";d.style.background=c.self;return d;}
    if(m.kind==="stop"){d.className="stop";d.style.background=c.background;d.style.borderColor=c.text;return d;}
    var pickup=m.kind==="pickup";d.className="pin";
    if(m.label){var p=document.createElement("div");p.className="pill"+(m.value?"":" one");
      p.style.background=pickup?c.primary:c.text;p.style.color=pickup?c.onPrimary:c.background;
      p.innerHTML='<div class="l"></div>'+(m.value?'<div class="v"></div>':"");
      p.querySelector(".l").textContent=m.label;if(m.value)p.querySelector(".v").textContent=m.value;d.appendChild(p);}
    var dot=document.createElement("div");dot.className="dot";dot.style.borderColor=pickup?c.primary:c.text;dot.style.background=c.background;d.appendChild(dot);
    return d;
  }
  function syncMarkers(s){
    var seen={};
    s.markers.forEach(function(m){
      var key=m.id+"|"+m.kind+"|"+(m.label||"")+"|"+(m.value||"");seen[m.id]=1;
      var cur=markers[m.id];
      if(cur&&cur.key!==key){cur.marker.remove();cur=null;}
      if(!cur){var anchor=(m.kind==="car"||m.kind==="self"||m.kind==="stop")?"center":"bottom";
        cur={key:key,marker:new maplibregl.Marker({element:el(m,s.colors),anchor:anchor}).setLngLat([m.at.lng,m.at.lat]).addTo(map)};markers[m.id]=cur;}
      else cur.marker.setLngLat([m.at.lng,m.at.lat]);
    });
    Object.keys(markers).forEach(function(id){if(!seen[id]){markers[id].marker.remove();delete markers[id];}});
  }
  function syncRoute(s){
    var data={type:"Feature",geometry:{type:"LineString",coordinates:s.route.map(function(p){return[p.lng,p.lat];})}};
    var src=map.getSource("route");
    if(src)src.setData(data);
    else{map.addSource("route",{type:"geojson",data:data});
      map.addLayer({id:"route",type:"line",source:"route",layout:{"line-cap":"round","line-join":"round"},paint:{"line-color":s.colors.primary,"line-width":5}});}
  }
  function syncCamera(s){
    var key=JSON.stringify([s.fit,s.centre,s.zoom,s.padding,s.recenterKey]);
    if(key===lastCamera)return;lastCamera=key;
    var pad=s.padding;
    if(s.fit.length>1){var b=new maplibregl.LngLatBounds();s.fit.forEach(function(p){b.extend([p.lng,p.lat]);});
      map.fitBounds(b,{padding:{top:pad.top+40,bottom:pad.bottom+40,left:pad.left+48,right:pad.right+48},duration:900,maxZoom:16});}
    else if(s.zoom){map.flyTo({center:[s.centre.lng,s.centre.lat],zoom:s.zoom,padding:pad,duration:1400,curve:1.2});}
    else{map.easeTo({center:[s.centre.lng,s.centre.lat],padding:pad,duration:500});}
  }
  // Keep the OpenStreetMap credit visible above sheets and floating bars (licence requirement).
  function syncAttribution(s){var c=document.querySelectorAll(".maplibregl-ctrl-bottom-left,.maplibregl-ctrl-bottom-right");
    for(var i=0;i<c.length;i++)c[i].style.bottom=(s.padding.bottom)+"px";}
  function apply(){if(!state||!loaded)return;syncRoute(state);syncMarkers(state);syncCamera(state);syncAttribution(state);}
  window.__update=function(s){state=s;apply();};
  map.on("load",function(){loaded=true;apply();post({type:"ready"});});
  map.on("moveend",function(e){if(!e.originalEvent)return;var c=map.getCenter();post({type:"centre",lat:c.lat,lng:c.lng});});
  map.on("error",function(e){post({type:"error",message:String(e&&e.error&&e.error.message||e)});});
}
  loadLib(0);
})();
</script></body></html>`;
}
