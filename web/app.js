const $ = (id) => document.getElementById(id);
let source = null, image = null, layout = null, drawnRequest = null;
let offsetX = 0, offsetY = 0, skipped = new Map(), version = 0, timer, busy = false;
let transform = null;
const canvas = $('preview'), stage = $('stage'), ctx = canvas.getContext('2d');
const call = (method, ...args) => window.mygo.call(`PosterService.${method}`, ...args);
function status(message, error = false) { $('status').textContent = message; $('status').classList.toggle('error', error); }
function request() {
 return {sourceID:source.id, width:$('width').valueAsNumber, height:$('height').valueAsNumber,
  zoom:Number($('zoom').value), offsetX, offsetY, orientation:$('orientation').value,
  printMap:$('map').checked, skipped:[...skipped.values()]};
}
function controls() {
 $('open').disabled = busy;
 $('settings').disabled = busy || !source;
 $('export').disabled = busy || !layout || layout.printed === 0;
}
function update() {
 const current = ++version;
 clearTimeout(timer); layout = null; drawnRequest = null; controls(); draw();
 $('zoomValue').textContent = `${Math.round(Number($('zoom').value)*100)}%`;
 $('offset').textContent = `偏移 X ${offsetX} / Y ${offsetY} mm`;
 if (!source) return;
 const req = request();
 if (![req.width,req.height].every(v => Number.isFinite(v) && v > 0)) {
  $('count').textContent = '尺寸无效'; status('请输入大于零的宽度和高度。', true); return;
 }
 timer = setTimeout(async () => {
  try {
   const next = await call('Preview', req);
   if (current !== version) return;
   layout = next; drawnRequest = req;
   $('count').textContent = `${next.printed} 张 A4${req.printMap ? ' + 1 张拼接地图' : ''}`;
   status(next.printed ? '点击图块可取消打印。导出后请以「实际大小 / 100%」打印。' : '没有可打印的图块，请调整图片位置或恢复图块。');
   controls(); draw();
  } catch (error) { if (current === version) { $('count').textContent = '无法预览'; status(String(error), true); } }
 }, 70);
}
function toggle(t) {
 if (busy) return;
 const key = `${t.col},${t.row}`;
 if (skipped.has(key)) skipped.delete(key); else skipped.set(key, {col:t.col,row:t.row});
 update();
}
function draw() {
 const width = stage.clientWidth, height = stage.clientHeight, dpr = window.devicePixelRatio || 1;
 canvas.width = Math.round(width*dpr); canvas.height = Math.round(height*dpr);
 ctx.setTransform(dpr,0,0,dpr,0,0); ctx.clearRect(0,0,width,height); transform = null;
 if (!layout || !image || !drawnRequest) return;
 const req = drawnRequest, scale = Math.min((width-64)/req.width,(height-64)/req.height);
 if (scale <= 0) return;
 const x = (width-req.width*scale)/2, y = (height-req.height*scale)/2;
 transform = {x,y,scale};
 ctx.save(); ctx.translate(x,y); ctx.scale(scale,scale);
 ctx.fillStyle = 'white'; ctx.fillRect(0,0,req.width,req.height);
 ctx.beginPath(); ctx.rect(0,0,req.width,req.height); ctx.clip();
 const dest = layout.image; ctx.drawImage(image,dest.x,dest.y,dest.width,dest.height);
 ctx.lineWidth = 1/scale; ctx.strokeStyle = '#4285df';
 ctx.beginPath();
 for(let px=layout.page.width;px<req.width;px+=layout.page.width){ctx.moveTo(px,0);ctx.lineTo(px,req.height);}
 for(let py=layout.page.height;py<req.height;py+=layout.page.height){ctx.moveTo(0,py);ctx.lineTo(req.width,py);}
 ctx.stroke();
 for(const tile of layout.tiles){
  const b=tile.bounds;
  if(tile.skipped){ctx.fillStyle='rgba(216,55,55,.30)';ctx.fillRect(b.x,b.y,b.width,b.height);}
  ctx.font=`${11/scale}px system-ui`;ctx.fillStyle=tile.skipped?'#a02222':'#1c5186';
  ctx.fillText(`${tile.col+1},${tile.row+1}`,b.x+6/scale,b.y+15/scale);
 }
 ctx.restore(); ctx.strokeStyle='#283c30';ctx.lineWidth=1.5;ctx.strokeRect(x,y,req.width*scale,req.height*scale);
}
canvas.addEventListener('click', event => {
 if (!transform || !layout || busy) return;
 const bounds=canvas.getBoundingClientRect(), {x,y,scale}=transform;
 const px=(event.clientX-bounds.left-x)/scale, py=(event.clientY-bounds.top-y)/scale;
 const tile=layout.tiles.find(t=>px>=t.bounds.x && py>=t.bounds.y && px<t.bounds.x+t.bounds.width && py<t.bounds.y+t.bounds.height);
 if(tile) toggle(tile);
});
$('open').addEventListener('click', async () => {
 busy=true;controls();
 try {
  const result=await call('OpenImage');
  if(result){
   const nextImage=new Image(); nextImage.src=result.dataURL; await nextImage.decode();
   source=result;image=nextImage;offsetX=offsetY=0;skipped.clear();
   $('source').textContent=`${result.name} · ${result.width} × ${result.height} 像素`;
   $('empty').hidden=true;update();
  }
 } catch(error){status(String(error),true);} finally {busy=false;controls();}
});
$('export').addEventListener('click', async () => {
 if (!drawnRequest || busy) return;
 busy=true;controls();status('正在导出 PDF…');
 try {const output=await call('Export',drawnRequest);status(output?`已导出：${output}。打印请选择「实际大小 / 100%」。`:'已取消导出。');}
 catch(error){status(String(error),true);}finally{busy=false;controls();}
});
for(const id of ['width','height','zoom','orientation','map']) $(id).addEventListener('input',update);
for(const button of document.querySelectorAll('[data-dx]')) button.addEventListener('click',()=>{offsetX+=Number(button.dataset.dx);offsetY+=Number(button.dataset.dy);update();});
$('reset').addEventListener('click',()=>{offsetX=offsetY=0;update();});
new ResizeObserver(draw).observe(stage);
if(!window.mygo){status('请使用 go run . 启动桌面应用。',true);$('open').disabled=true;}
