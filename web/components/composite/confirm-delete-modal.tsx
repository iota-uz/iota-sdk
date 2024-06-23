import * as Icons from '@phosphor-icons/react';
import { Button } from '@/components/ui/button';
import Image from 'next/image';

export type Props = {
    open: boolean;
    title: string;
    subtitle: string;
    onConfirm: () => void;
    onClose: () => void;
}

export default function ConfirmDeleteModal({
  open, onConfirm, title, subtitle, onClose,
}: Props) {
  if (!open) {
    return null;
  }
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-sm rounded-2xl bg-white p-6 shadow-lg dark:bg-gray-900 relative">
        <Icons.X className="absolute -top-10 right-0 text-white cursor-pointer" onClick={onClose} size={20} weight="bold" />
        <div className="space-y-10">
          <div className="flex flex-col items-center gap-5">
            <Image
              alt="Profile Picture"
              className="rounded-full"
              height={94}
              width={94}
              src="/svg/delete-placeholder.svg"
              style={{
                aspectRatio: '80/80',
                objectFit: 'cover',
              }}
            />
            <div className="text-center max-w-72 space-y-4">
              <h3 className="text-lg font-semibold">
                {title}
              </h3>
              <p className="text-gray-600 dark:text-gray-400">
                {subtitle}
              </p>
            </div>
          </div>
          <div className="flex flex-col gap-2">
            <Button className="bg-primary-100 hover:bg-primary-200 text-gray-700" onClick={onClose}>
              Отмена
            </Button>
            <Button onClick={() => {
              onConfirm();
              onClose();
            }}
            >
              Удалить
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
