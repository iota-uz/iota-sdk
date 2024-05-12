'use client';
import {Card, CardContent, CardFooter} from "@/components/ui/card"
import {Label} from "@/components/ui/label"
import {Input} from "@/components/ui/input"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import {Button} from "@/components/ui/button"

export default function Component() {
    return (
        <div className="container">
            <Card className="w-full h-full max-w-none shadow-none mt-4">
                <CardContent className="space-y-6 p-6 md:p-8 lg:p-10">
                    <div className="space-y-2">
                        <Label htmlFor="profile-picture">Фотография профиля</Label>
                        <div className="flex items-center space-x-4">
                            <img
                                alt="Profile Picture"
                                className="rounded-full"
                                height={80}
                                src="/placeholder.svg"
                                style={{
                                    aspectRatio: "80/80",
                                    objectFit: "cover",
                                }}
                                width={80}
                            />
                        </div>
                    </div>
                    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                        <div className="space-y-2">
                            <Label htmlFor="firstname">Имя</Label>
                            <Input id="firstname" placeholder="Введите ваше имя"/>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="lastname">Фамилия</Label>
                            <Input id="lastname" placeholder="Введите вашу фамилию"/>
                        </div>
                    </div>
                    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                        <div className="space-y-2">
                            <Label htmlFor="email">Электронная почта</Label>
                            <Input id="email" placeholder="Введите вашу электронную почту" type="email"/>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="password">Пароль</Label>
                            <Input id="password" placeholder="Введите пароль" type="password"/>
                        </div>
                    </div>
                    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                        <div className="space-y-2">
                            <Label htmlFor="role">Роль</Label>
                            <Select>
                                <SelectTrigger>
                                    <SelectValue placeholder="Выберите роль"/>
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="admin">Администратор</SelectItem>
                                    <SelectItem value="manager">Менеджер</SelectItem>
                                    <SelectItem value="user">Пользователь</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    </div>
                </CardContent>
                <CardFooter className="flex justify-between p-6 md:p-8 lg:p-10">
                    <Button className="text-red-500 hover:bg-red-500 hover:text-white" variant="outline">
                        Удалить
                    </Button>
                    <Button className="ml-auto" type="submit">
                        Сохранить изменения
                    </Button>
                </CardFooter>
            </Card>
        </div>
    )
}