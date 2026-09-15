import { splitProps, type JSX } from 'solid-js'
import { Select, type SelectProps } from '../forms/Select'
import { callHandler } from '../internal/events'

export const countryCodes = 'AF,AL,DZ,AD,AO,AG,AR,AM,AU,AT,AZ,BS,BH,BD,BB,BY,BE,BZ,BJ,BT,BO,BA,BW,BR,BN,BG,BF,BI,CV,KH,CM,CA,CF,TD,CL,CN,CO,KM,CG,CD,CR,HR,CU,CY,CZ,DK,DJ,DM,DO,EC,EG,SV,GQ,ER,EE,SZ,ET,FJ,FI,FR,GA,GM,GE,DE,GH,GR,GD,GT,GN,GW,GY,HT,HN,HU,IS,IN,IDN,IR,IQ,IE,IL,IT,JM,JP,JO,KZ,KE,KI,KP,KR,XK,KW,KG,LA,LV,LB,LS,LR,LY,LI,LT,LU,MG,MW,MY,MV,ML,MT,MH,MR,MU,MX,FM,MD,MC,MN,ME,MA,MZ,MM,NA,NR,NP,NL,NZ,NI,NE,NG,MK,NO,OM,PK,PW,PS,PA,PG,PY,PE,PH,PL,PT,QA,RO,RU,RW,KN,LC,VC,WS,SM,ST,SA,SN,RS,SC,SL,SG,SK,SI,SB,SO,ZA,SS,ES,LK,SD,SR,SE,CH,SY,TJ,TZ,TH,TL,TG,TO,TT,TN,TR,TM,TV,UG,UA,AE,GB,US,UY,UZ,VU,VA,VE,VN,YE,ZM,ZW'.split(',') as readonly string[]

export interface CountryOption {
  code: string
  label: string
}

export interface CountriesSelectProps extends Omit<SelectProps, 'options' | 'prefix'> {
  countries?: readonly string[]
  countryLabels?: Partial<Record<string, string>>
  locale?: string
  getCountryLabel?: (code: string, locale: string) => string
  onValueChange?: (value: string) => void
}

export function getCountryOptions(
  locale = 'en',
  countries: readonly string[] = countryCodes,
  countryLabels: Partial<Record<string, string>> = {},
  getCountryLabel?: (code: string, locale: string) => string,
): CountryOption[] {
  let displayNames: Intl.DisplayNames | undefined
  try {
    displayNames = new Intl.DisplayNames([locale], { type: 'region' })
  } catch {
    displayNames = new Intl.DisplayNames(['en'], { type: 'region' })
  }
  return countries.map((code) => {
    let label = countryLabels[code]
    if (!label && getCountryLabel) label = getCountryLabel(code, locale)
    if (!label) {
      try { label = displayNames.of(code) }
      catch { label = code }
    }
    return { code, label: label || code }
  })
}

export function CountriesSelect(props: CountriesSelectProps) {
  const [local, selectProps] = splitProps(props, ['countries', 'countryLabels', 'locale', 'getCountryLabel', 'onValueChange', 'onChange'])
  const options = () => getCountryOptions(local.locale, local.countries, local.countryLabels, local.getCountryLabel)
  const handleChange: JSX.EventHandler<HTMLSelectElement, Event> = (event) => {
    local.onValueChange?.(event.currentTarget.value)
    callHandler(local.onChange, event)
  }
  return (
    <Select
      {...selectProps}
      onChange={handleChange}
      options={options().map((country) => ({ value: country.code, label: country.label }))}
    />
  )
}

export const CountrySelect = CountriesSelect
