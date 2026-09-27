import { Input, type InputPresetProps } from '../forms/Input'

export type PhoneInputProps = InputPresetProps

export function PhoneInput(props: PhoneInputProps) {
  return <Input {...props} type="tel" inputmode={props.inputmode ?? 'tel'} autocomplete={props.autocomplete ?? 'tel'} />
}
