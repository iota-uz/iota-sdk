import { CSSProperties } from 'react';
import Image from "next/image";

/**
 * v0 by Vercel.
 * @see https://v0.dev/t/za4LHM1OzaN
 * Documentation: https://v0.dev/docs#integrating-generated-code-into-your-nextjs-app
 */
export default function PictureInput(props: React.ComponentProps<'input'>) {
  const style: CSSProperties = {
    backgroundImage: 'url("data:image/svg+xml,%3csvg width=\'100%25\' height=\'100%25\' xmlns=\'http://www.w3.org/2000/svg\'%3e%3crect width=\'100%25\' height=\'100%25\' fill=\'none\' rx=\'10\' ry=\'10\' stroke=\'%23DEE6FBFF\' stroke-width=\'4\' stroke-dasharray=\'20\' stroke-dashoffset=\'0\' stroke-linecap=\'round\'/%3e%3c/svg%3e")',
    borderRadius: '10px',
  };

  return (
    <div
      className="flex justify-center items-center w-full h-32 bg-gray-50 rounded-lg cursor-pointer dark:bg-gray-800"
      style={style}
    >
      <label className="flex flex-col items-center justify-center w-full h-full" htmlFor="file-input">
        <Image
          alt="Profile Picture"
          className="rounded-full"
          height={65}
          width={65}
          src="/svg/placeholder.svg"
          style={{
            aspectRatio: '80/80',
            objectFit: 'cover',
          }}
        />
        <span className="mt-2 text-sm font-medium text-gray-500 dark:text-gray-400 cursor-pointer">
          Перетащить или
          {' '}
          <span className="text-primary-600 underline underline-offset-2">выбрать</span>
        </span>
        <input
          {...props}
          className="sr-only"
          accept="image/*"
          type="file"
        />
      </label>
    </div>
  );
}
