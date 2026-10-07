import {useEffect,useRef} from 'react'

// Render at low resolution and scale without smoothing.
export function PixelScene(){
 const ref=useRef<HTMLCanvasElement>(null)
 useEffect(()=>{
  const canvas=ref.current,context=canvas?.getContext('2d')
  if(!canvas||!context)return
  const motion=matchMedia('(prefers-reduced-motion: reduce)')
  let frame=0,position=0,target=0,disposed=false
  const paint=()=>{
   frame=0;if(disposed)return
   position=motion.matches?0:position+(target-position)*.16
   const css=getComputedStyle(document.documentElement)
   const dark=css.getPropertyValue('--pixel-dark').trim(),mid=css.getPropertyValue('--pixel-mid').trim(),light=css.getPropertyValue('--pixel-light').trim()
   context.clearRect(0,0,128,64);context.imageSmoothingEnabled=false
   const block=(x:number,y:number,w:number,h:number,color:string)=>{context.fillStyle=color;context.fillRect(Math.round(x),Math.round(y),w,h)}
   // Island and coin stacks.
   block(14,51,100,2,mid);block(22,45,84,6,light);block(30,39,68,6,light)
   for(let i=0;i<7;i++)block(34+i*8,48,4,2,mid)
   const stack=(x:number,y:number,count:number)=>{
    for(let i=0;i<count;i++){
     block(x,y-i*5,18,4,mid);block(x+2,y-i*5,14,2,light);block(x+16,y-i*5+1,2,3,dark)
    }
   }
   stack(34,38,3);stack(56,38,5);stack(78,38,7)
   // Sprouts follow the cursor.
   const sway=Math.round(position*2)
   block(21,27,2,18,dark);block(16+sway,24,7,4,mid);block(23+sway,20,7,4,mid);block(19+sway,28,4,3,light)
   block(103-sway,17,2,18,dark);block(98-sway,15,7,4,mid);block(105-sway,11,7,4,mid)
   block(97,33,14,4,mid);block(99,37,10,5,dark)
   // Static stars.
   block(16,10,2,6,mid);block(14,12,6,2,mid);block(89,5,2,2,light)
   block(48,10,2,2,mid);block(112,27,2,2,light)
   if(!motion.matches&&Math.abs(target-position)>.01)frame=requestAnimationFrame(paint)
  }
  const schedule=()=>{if(!frame&&!disposed)frame=requestAnimationFrame(paint)}
  const move=(event:PointerEvent)=>{if(motion.matches)return;const box=canvas.getBoundingClientRect();target=(event.clientX-box.left)/box.width*2-1;schedule()}
  const reset=()=>{target=0;schedule()}
  const scroll=()=>{if(motion.matches)return;target=Math.min(1,scrollY/400);schedule()}
  const theme=new MutationObserver(schedule);theme.observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']})
  canvas.addEventListener('pointermove',move);canvas.addEventListener('pointerleave',reset)
  window.addEventListener('scroll',scroll,{passive:true});motion.addEventListener('change',reset)
  paint()
  return()=>{disposed=true;cancelAnimationFrame(frame);theme.disconnect();canvas.removeEventListener('pointermove',move);canvas.removeEventListener('pointerleave',reset);window.removeEventListener('scroll',scroll);motion.removeEventListener('change',reset)}
 },[])
 return <canvas ref={ref} className="pixel-scene" width={128} height={64} aria-hidden="true"/>
}
export function PixelMark(){return <svg viewBox="0 0 24 24" width="24" height="24" aria-hidden="true" shapeRendering="crispEdges"><path fill="currentColor" d="M2 14h5v8H2zm8-6h5v14h-5zm8-6h5v20h-5z"/></svg>}
