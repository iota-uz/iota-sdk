'use client';

import React, {createContext, useContext, useEffect, useState} from 'react';
import {useParams, useRouter} from 'next/navigation';
import {gql, useMutation, useQuery} from '@apollo/client';
import {Card, CardContent, CardFooter} from '@/components/ui/card';
import {Label} from '@/components/ui/label';
import {Input} from '@/components/ui/input';
import {Button} from '@/components/ui/button';
import PictureInput from '@/components/composite/picture-input';
import {CreateUser, User} from '@/src/__generated__/graphql';
import ConfirmDeleteModal from '@/components/composite/confirm-delete-modal';
import {useTranslations} from "next-intl";


type UserContextType = {
  t: ReturnType<typeof useTranslations<'User'>>;
}

const UserContext = createContext<UserContextType | null>(null);

const GET_USER = gql`
    query GetUser($id: ID!) {
        user(id: $id) {
            id
            firstName
            lastName
            email
        }
    }
`;

const CREATE_USER = gql`
    mutation CreateUser($input: CreateUser!) {
        createUser(input: $input) {
            id
        }
    }
`;

const UPDATE_USER = gql`
    mutation UpdateUser($id: ID!, $input: UpdateUser!) {
        updateUser(id: $id, input: $input) {
            id
        }
    }
`;

const DELETE_USER = gql`
    mutation RemoveUser($id: ID!) {
        deleteUser(id: $id)
    }
`;

type UserInfoProps = {
  user: Partial<User>;
  updateUser: (args: Partial<CreateUser>) => void;
}

function UserInfo({user, updateUser}: UserInfoProps) {
  const {t} = useContext(UserContext)!;

  return (
    <>
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="firstname">
            {t('firstName.label')}
          </Label>
          <Input
            id="firstname"
            placeholder={t('firstName.placeholder')}
            value={user.firstName}
            onChange={(e) => updateUser({firstName: e.target.value})}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="lastname">
            {t('lastName.label')}
          </Label>
          <Input
            id="lastname"
            placeholder={t('lastName.placeholder')}
            value={user.lastName}
            onChange={(e) => updateUser({lastName: e.target.value})}
          />
        </div>
      </div>
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="email">
            {t('email.label')}
          </Label>
          <Input
            id="email"
            placeholder={t('email.placeholder')}
            type="email"
            value={user.email}
            onChange={(e) => updateUser({email: e.target.value})}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">
            {t('password.label')}
          </Label>
          <Input
            id="password"
            placeholder={t('password.placeholder')}
            type="password"
            onChange={(e) => updateUser({password: e.target.value})}
          />
        </div>
      </div>
    </>
  );
}

type PageFooterProps = {
  onDelete?: () => void;
  onSave: () => Promise<any>;
}

function PageFooter({onDelete, onSave}: PageFooterProps) {
  const [open, setOpen] = useState(false);
  const {t} = useContext(UserContext)!;

  if (onDelete) {
    return (
      <>
        <ConfirmDeleteModal
          open={open}
          onConfirm={onDelete}
          title="Удалить пользователя"
          subtitle="Вы действительно хотите удалить пользователя?"
          onClose={() => setOpen(false)}
        />
        <Button
          className="text-red-500 hover:bg-red-500 bg-red-100 hover:text-white"
          onClick={() => setOpen(true)}
        >
          {t('delete')}
        </Button>
        <Button
          className="ml-5"
          type="submit"
          onClick={onSave}
        >
          {t('save')}
        </Button>
      </>
    );
  }
  return (
    <Button
      type="submit"
      onClick={onSave}
    >
      {t('add')}
    </Button>
  );
}

export function NewUser() {
  const router = useRouter();
  const [user, setUser] = useState<Partial<User>>({
    firstName: '',
    lastName: '',
    email: '',
  });
  const [createUser] = useMutation(CREATE_USER);

  function updateUser(args: Partial<User>) {
    setUser({...user, ...args});
  }

  async function onSave() {
    await createUser({
      variables: {input: user},
    });
    return router.push('/users');
  }

  return (
    <div className="container">
      <Card className="w-full h-full max-w-none shadow-none mt-5">
        <CardContent className="space-y-6 p-6 md:p-8 lg:p-10">
          <PictureInput/>
          <UserInfo user={user} updateUser={updateUser}/>
        </CardContent>
        <CardFooter className="flex justify-end p-6 md:p-8 lg:p-10">
          <PageFooter onSave={onSave}/>
        </CardFooter>
      </Card>
    </div>
  );
}

export function EditUser() {
  const router = useRouter();
  const params = useParams();
  const [user, setUser] = useState<Partial<User>>({
    firstName: '',
    lastName: '',
    email: '',
  });
  const {loading, data} = useQuery(GET_USER, {
    variables: {id: params.id},
  });
  useEffect(() => {
    if (data?.user) {
      setUser(data.user);
    }
  }, [data]);

  const [patchUser] = useMutation(UPDATE_USER);
  const [deleteUser] = useMutation(DELETE_USER);

  function updateUser(args: Partial<User>) {
    setUser({...user, ...args});
  }

  async function onSave() {
    await patchUser({variables: {id: user.id, input: user}});
    return router.push('/users');
  }

  async function onDelete() {
    await deleteUser({variables: {id: user.id}});
    return router.push('/users');
  }

  if (loading) {
    return (
      <div className="container">
        <div className="flex items-center justify-center h-screen">
          <div className="spinner spinner-primary"/>
        </div>
      </div>
    );
  }

  return (
    <div className="container">
      <Card className="w-full h-full max-w-none shadow-none mt-5">
        <CardContent className="space-y-6 p-6 md:p-8 lg:p-10">
          <PictureInput/>
          <UserInfo user={user} updateUser={updateUser}/>
        </CardContent>
        <CardFooter className="flex justify-end p-6 md:p-8 lg:p-10">
          <PageFooter
            onSave={onSave}
            onDelete={onDelete}
          />
        </CardFooter>
      </Card>
    </div>
  );
}

export default function Component() {
  const params = useParams();
  const t = useTranslations('User');
  const context = {
    t
  };

  if (params.id === 'new') {
    return (
      <UserContext.Provider value={context}>
        <NewUser/>
      </UserContext.Provider>
    );
  }
  return (
    <UserContext.Provider value={context}>
      <EditUser/>
    </UserContext.Provider>
  );
}
