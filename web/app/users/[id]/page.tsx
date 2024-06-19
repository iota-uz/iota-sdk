'use client';
import React, {useEffect, useState} from 'react';
import {useParams, useRouter} from 'next/navigation';
import {gql, useMutation, useQuery} from '@apollo/client';
import {Card, CardContent, CardFooter} from '@/components/ui/card';
import {Label} from '@/components/ui/label';
import {Input} from '@/components/ui/input';
import {Button} from '@/components/ui/button';
import PictureInput from '@/components/composite/picture-input';
import {User} from '@/src/__generated__/graphql';
import ConfirmDeleteModal from "@/components/composite/confirm-delete-modal";

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
    mutation CreateUser($data: CreateUser!) {
        createUser(input: $data) {
            id
        }
    }
`;

const UPDATE_USER = gql`
    mutation UpdateUser($id: ID!, $data: UpdateUser!) {
        updateUser(id: $id, input: $data) {
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
    user: User;
    updateUser: (args: Partial<User>) => void;
}

const UserInfo = ({user, updateUser}: UserInfoProps) => (
    <>
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <div className="space-y-2">
                <Label htmlFor="firstname">Имя</Label>
                <Input
                    id="firstname"
                    placeholder="Введите ваше имя"
                    value={user.firstName}
                    onChange={(e) => updateUser({firstName: e.target.value})}
                />
            </div>
            <div className="space-y-2">
                <Label htmlFor="lastname">Фамилия</Label>
                <Input
                    id="lastname"
                    placeholder="Введите вашу фамилию"
                    value={user.lastName}
                    onChange={(e) => updateUser({lastName: e.target.value})}
                />
            </div>
        </div>
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <div className="space-y-2">
                <Label htmlFor="email">Электронная почта</Label>
                <Input
                    id="email"
                    placeholder="Введите вашу электронную почту"
                    type="email"
                    value={user.email}
                    onChange={(e) => updateUser({email: e.target.value})}
                />
            </div>
            <div className="space-y-2">
                <Label htmlFor="password">Пароль</Label>
                <Input
                    id="password"
                    placeholder="Введите пароль"
                    type="password"
                    onChange={(e) => updateUser({password: e.target.value})}
                />
            </div>
        </div>
    </>
);

type PageFooterProps = {
    onDelete?: () => void;
    onSave: () => Promise<any>;
}

const PageFooter = ({onDelete, onSave}: PageFooterProps) => {
    const [open, setOpen] = useState(false);
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
                    Удалить
                </Button>
                <Button
                    className="ml-5"
                    type="submit"
                    onClick={onSave}
                >
                    Сохранить
                </Button>
            </>
        )
    }
    return (
        <Button type="submit">
            Добавить
        </Button>
    )
}

export default function Component() {
    const router = useRouter();
    const params = useParams();
    const [user, setUser] = useState<User>({
        id: '',
        firstName: '',
        lastName: '',
        email: '',
        createdAt: '',
        updatedAt: '',
    });
    let loading = false;
    if (params.id !== 'new') {
        const queryResult = useQuery(GET_USER, {
            variables: {id: params.id},
        });
        loading = queryResult.loading;
        useEffect(() => {
            if (queryResult.data?.user) {
                setUser(queryResult.data.user);
            }
        }, [queryResult.data]);
    }
    const updateUser = (args: Partial<User>) => setUser({...user, ...args});
    const [createUser] = useMutation(CREATE_USER);
    const [patchUser] = useMutation(UPDATE_USER);
    const [deleteUser] = useMutation(DELETE_USER);

    const onSave = (): Promise<any> => {
        const {id, ...rest} = user;
        if (id) {
            return patchUser({variables: {id, data: rest}})
        }
        return createUser({
            variables: {data: user}
        });
    }

    const onDelete = async () => {
        const r = await deleteUser({variables: {id: user.id}});
        router.push('/users');
        return r;
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