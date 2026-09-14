import { For, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { createControllable } from './state'

export interface ToggleOption {
  value: string
  label: JSX.Element
  disabled?: boolean
}

export type ToggleSize = 'sm' | 'md'
export type ToggleRounded = 'none' | 'rounded' | 'smooth' | 'curved' | 'full'
export type ToggleAlignment = 'start' | 'center' | 'end'

export interface ToggleProps extends JSX.HTMLAttributes<HTMLDivElement> {
  options: readonly ToggleOption[]
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  name?: string
  form?: string
  size?: ToggleSize
  rounded?: ToggleRounded
  alignment?: ToggleAlignment
}

const roundedClasses: Record<ToggleRounded, string> = {
  none: '', rounded: 'tabs-rounded', smooth: 'tabs-smooth', curved: 'tabs-curved', full: 'tabs-full',
}

export function Toggle(props: ToggleProps) {
  const [local, native] = splitProps(props, ['class', 'options', 'value', 'defaultValue', 'onValueChange', 'name', 'form', 'size', 'rounded', 'alignment'])
  const [value, setValue] = createControllable(() => local.value, local.defaultValue ?? local.options[0]?.value ?? '')
  const choose = (option: ToggleOption) => {
    if (option.disabled) return
    setValue(option.value)
    local.onValueChange?.(option.value)
  }
  return (
    <div {...native} class={classes(
      'tab-slider',
      local.size === 'sm' ? 'tabs-sm' : local.size === 'md' ? 'tabs-md' : undefined,
      local.options.length === 2 ? 'tabs-two-slots' : 'tabs-three-slots',
      roundedClasses[local.rounded ?? 'none'],
      local.alignment === 'center' ? 'tabs-centered' : local.alignment === 'end' ? 'tabs-end' : undefined,
      local.class,
    )}>
      <div class="tab-slider-inner">
        <div class="tab-slider-track" role="group">
          <For each={local.options}>{(option) => (
            <button
              type="button"
              class={classes('tab-slider-item cursor-pointer', value() === option.value && 'tab-active')}
              disabled={option.disabled}
              aria-pressed={value() === option.value}
              onClick={() => choose(option)}
            >
              {option.label}
            </button>
          )}</For>
          <div class="tab-slider-naver" />
        </div>
      </div>
      <input type="hidden" name={local.name} form={local.form} value={value()} />
    </div>
  )
}

