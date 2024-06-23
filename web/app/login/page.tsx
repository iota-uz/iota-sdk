import {useTranslations} from "next-intl";
import Image from "next/image";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {Button} from "@/components/ui/button";

export default function Page() {
  const t = useTranslations('Login');
  return (
    <div className="grid grid-cols-2 h-screen">
      <div className="flex flex-col gap-16 justify-center items-center text-left">
        <h1 className="text-2xl text-gray-950">
          {t('title')}
        </h1>
        <form className="flex flex-col gap-7 w-2/3">
          <div>
            <Label>
              {t('email.label')}
            </Label>
            <Input
              type="email"
              name="email"
              placeholder={t('email.placeholder')}
            />
          </div>
          <div>
            <Label>
              {t('password.label')}
            </Label>
            <Input
              type="password"
              name="password"
              placeholder={t('password.placeholder')}
            />
          </div>
          <Button>
            {t('submit')}
          </Button>
        </form>
      </div>
      <div className="bg-primary-700">
        <Image
          className="w-full h-full object-cover"
          src="/img/login.png"
          alt="Login"
          width={800}
          height={800}
        />
      </div>
    </div>
  )
}
