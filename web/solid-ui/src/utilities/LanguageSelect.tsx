import { createSignal, createUniqueId, For, Show, splitProps, type JSX } from 'solid-js'
import type { SelectProps } from '../forms/Select'
import { Field } from '../forms/Field'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'
import { CaretDownIcon } from '../internal/icons'

export interface SupportedLanguage {
  code: string
  verboseName: string
}

export const supportedLanguages: readonly SupportedLanguage[] = [
  { code: 'ru', verboseName: 'Русский' },
  { code: 'en', verboseName: 'English' },
  { code: 'uz', verboseName: "O'zbekcha" },
  { code: 'uz-Cyrl', verboseName: 'Ўзбекча' },
  { code: 'pt-BR', verboseName: 'Português (Brasil)' },
]

export interface LanguageSelectProps extends Omit<SelectProps, 'options' | 'prefix'> {
  languages?: readonly SupportedLanguage[]
  onValueChange?: (value: string) => void
}

export function LanguageSelect(props: LanguageSelectProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, ['languages', 'onValueChange', 'onChange', 'defaultValue', 'value', 'label', 'error', 'placeholder', 'wrapperClass', 'class', 'id', 'aria-describedby'])
  const [internalValue, setInternalValue] = createSignal(String(local.defaultValue ?? ''))
  const languages = () => local.languages?.length ? local.languages : supportedLanguages
  const value = () => local.value === undefined ? internalValue() : String(local.value)
  const id = () => local.id ?? generatedID
  const errorID = () => `${id()}-error`
  const handleChange: JSX.EventHandler<HTMLSelectElement, Event> = (event) => {
    if (local.value === undefined) setInternalValue(event.currentTarget.value)
    local.onValueChange?.(event.currentTarget.value)
    callHandler(local.onChange, event)
  }
  return <Field class={classes('shrink-0', local.wrapperClass)} label={local.label} labelFor={id()} error={local.error} errorId={errorID()} required={native.required}>
    <div class="w-full relative flex items-center">
      <select {...native} id={id()} class={classes('min-w-20 w-full appearance-none form-control form-control-input h-[2.6875rem] pr-8', local.class)} value={value()} onChange={handleChange} aria-invalid={local.error ? true : native['aria-invalid']} aria-describedby={[local['aria-describedby'], local.error ? errorID() : undefined].filter(Boolean).join(' ') || undefined}>
        <Show when={local.placeholder}><option value="" disabled selected={!value()}>{local.placeholder}</option></Show>
        <For each={languages()}>{(language) => <option value={language.code} selected={value() === language.code}>{language.verboseName}</option>}</For>
      </select>
      <CaretDownIcon class="absolute top-1/2 right-3 -translate-y-1/2 pointer-events-none" size={16} />
    </div>
  </Field>
}
