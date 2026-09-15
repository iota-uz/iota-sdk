import { createSignal, splitProps, type JSX } from 'solid-js'
import { EmptyTableIllustration } from '../data/EmptyState'
import { classes } from '../internal/classes'

export interface EmptyTableProps extends JSX.SvgSVGAttributes<SVGSVGElement> {
  width: number | string
  height: number | string
}

export function EmptyTable(props: EmptyTableProps) {
  return <EmptyTableIllustration {...props} />
}

export interface AccentColorProps extends Omit<JSX.LabelHTMLAttributes<HTMLLabelElement>, 'onChange'> {
  name: string
  value: string
  color: string
  form?: string
  checked?: boolean
  defaultChecked?: boolean
  disabled?: boolean
  onCheckedChange?: (checked: boolean, value: string) => void
  inputProps?: Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'type' | 'name' | 'value' | 'form'>
}

export function AccentColor(props: AccentColorProps) {
  const [local, native] = splitProps(props, ['name', 'value', 'color', 'form', 'checked', 'defaultChecked', 'disabled', 'onCheckedChange', 'inputProps', 'class', 'style'])
  const [internalChecked, setInternalChecked] = createSignal(local.defaultChecked ?? false)
  const checked = () => local.checked ?? internalChecked()
  const style = () => typeof local.style === 'string' ? `${local.style};--color:${local.color}` : { ...local.style, '--color': local.color }
  return <label {...native} class={classes('accent-color-card cursor-pointer rounded-xl border border-subtle max-w-xs flex flex-col items-center gap-3 p-3 pb-4 duration-300 has-[input:checked]:border-[var(--color)]', local.disabled && 'opacity-50 cursor-not-allowed', local.class)} style={style()}>
    <input {...local.inputProps} type="radio" name={local.name} value={local.value} form={local.form} class={classes('peer appearance-none absolute', local.inputProps?.class)} checked={checked()} disabled={local.disabled} onChange={(event) => { if (local.checked === undefined) setInternalChecked(event.currentTarget.checked); local.onCheckedChange?.(event.currentTarget.checked, local.value) }} />
    <div class="rounded-xl border-2 border-default flex items-center justify-center p-4 bg-surface-100 w-full duration-300 text-300 peer-checked:text-[var(--color)]">
      <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="68" height="60" fill="none"><rect width="8" height="44" y="16" fill="currentColor" rx="4"/><rect width="8" height="60" x="20" fill="currentColor" rx="4"/><rect width="8" height="44" x="40" y="16" fill="currentColor" rx="4"/><rect width="8" height="60" x="60" fill="currentColor" rx="4"/></svg>
    </div>
    <div class="w-4 h-4 rounded-full bg-[var(--color)] relative outline outline-offset-2 outline-border-default duration-300 peer-checked:outline-[var(--color)]" />
  </label>
}

export interface HandLoaderProps extends JSX.HTMLAttributes<HTMLDivElement> {
  skinColor?: string
  tapSpeed?: string
  tapStagger?: string
  label?: string
}

export function HandLoader(props: HandLoaderProps) {
  const [local, native] = splitProps(props, ['skinColor', 'tapSpeed', 'tapStagger', 'label', 'class', 'style'])
  const style = () => {
    const variables = { '--skin-color': local.skinColor ?? '#E4C560', '--tap-speed': local.tapSpeed ?? '0.6s', '--tap-stagger': local.tapStagger ?? '0.1s' }
    return typeof local.style === 'string' ? `${local.style};${Object.entries(variables).map(([name, value]) => `${name}:${value}`).join(';')}` : { ...local.style, ...variables }
  }
  return <div {...native} role={native.role ?? 'status'} aria-label={local.label ?? 'Loading'} class={classes('🤚', local.class)} style={style()}>
    <div class="👉"/><div class="👉"/><div class="👉"/><div class="👉"/><div class="🌴"/><div class="👍"/>
    <style>{`.🤚{position:relative;width:80px;height:60px;margin-left:80px}.🤚:before{content:'';display:block;width:180%;height:75%;position:absolute;top:70%;right:20%;background-color:#000;border-radius:40px 10px;filter:blur(10px);opacity:.3}.🌴{display:block;width:100%;height:100%;position:absolute;inset:0;background-color:var(--skin-color);border-radius:10px 40px}.👍{position:absolute;width:120%;height:38px;background-color:var(--skin-color);bottom:-18%;right:1%;transform-origin:calc(100% - 20px) 20px;transform:rotate(-20deg);border-radius:30px 20px 20px 10px;border-bottom:2px solid rgba(0,0,0,.1);border-left:2px solid rgba(0,0,0,.1)}.👍:after{width:20%;height:60%;content:'';background-color:rgba(255,255,255,.3);position:absolute;bottom:-8%;left:5px;border-radius:60% 10% 10% 30%;border-right:2px solid rgba(0,0,0,.05)}.👉{position:absolute;width:80%;height:35px;background-color:var(--skin-color);bottom:32%;right:64%;transform-origin:100% 20px;animation-duration:calc(var(--tap-speed)*2);animation-timing-function:ease-in-out;animation-iteration-count:infinite;transform:rotate(10deg)}.👉:before{content:'';position:absolute;width:140%;height:30px;background-color:var(--skin-color);bottom:8%;right:65%;transform-origin:calc(100% - 20px) 20px;transform:rotate(-60deg);border-radius:20px}.👉:nth-child(1){animation-delay:0;filter:brightness(70%);animation-name:tap-upper-1}.👉:nth-child(2){animation-delay:var(--tap-stagger);filter:brightness(80%);animation-name:tap-upper-2}.👉:nth-child(3){animation-delay:calc(var(--tap-stagger)*2);filter:brightness(90%);animation-name:tap-upper-3}.👉:nth-child(4){animation-delay:calc(var(--tap-stagger)*3);filter:brightness(100%);animation-name:tap-upper-4}@keyframes tap-upper-1{0%,50%,100%{transform:rotate(10deg) scale(.4)}40%{transform:rotate(50deg) scale(.4)}}@keyframes tap-upper-2{0%,50%,100%{transform:rotate(10deg) scale(.6)}40%{transform:rotate(50deg) scale(.6)}}@keyframes tap-upper-3{0%,50%,100%{transform:rotate(10deg) scale(.8)}40%{transform:rotate(50deg) scale(.8)}}@keyframes tap-upper-4{0%,50%,100%{transform:rotate(10deg) scale(1)}40%{transform:rotate(50deg) scale(1)}}`}</style>
  </div>
}

export const Hand = HandLoader
