/* eslint-disable */
export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
};

export type AuthPayload = {
  __typename?: 'AuthPayload';
  token?: Maybe<Scalars['String']['output']>;
};

export type AvatarInput = {
  created_at?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  mimetype?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  path?: InputMaybe<Scalars['String']['input']>;
  size?: InputMaybe<Scalars['Float']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
  uploader_id?: InputMaybe<Scalars['Int']['input']>;
};

export type CategoryInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type CreateEmployeeInput = {
  avatar_id?: InputMaybe<Scalars['Int']['input']>;
  coefficient?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  first_name?: InputMaybe<Scalars['String']['input']>;
  hourly_rate?: InputMaybe<Scalars['Float']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  last_name?: InputMaybe<Scalars['String']['input']>;
  meta?: InputMaybe<MetaInput>;
  middle_name?: InputMaybe<Scalars['String']['input']>;
  phone?: InputMaybe<Scalars['String']['input']>;
  position?: InputMaybe<PositionInput>;
  position_id?: InputMaybe<Scalars['Int']['input']>;
  salary?: InputMaybe<Scalars['Float']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type CreateExpenseCategoryInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type CreateExpenseInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  category?: InputMaybe<CategoryInput>;
  category_id?: InputMaybe<Scalars['Int']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  date?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type CreatePositionInput = {
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type CreateTaskTypeInput = {
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  icon?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type CreateUserInput = {
  avatar?: InputMaybe<AvatarInput>;
  avatar_id?: InputMaybe<Scalars['Int']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  employee_id?: InputMaybe<Scalars['Int']['input']>;
  first_name?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  last_action?: InputMaybe<Scalars['String']['input']>;
  last_ip?: InputMaybe<Scalars['String']['input']>;
  last_login?: InputMaybe<Scalars['String']['input']>;
  last_name?: InputMaybe<Scalars['String']['input']>;
  middle_name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type Employee = {
  __typename?: 'Employee';
  avatar_id?: Maybe<Scalars['Int']['output']>;
  coefficient?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  first_name?: Maybe<Scalars['String']['output']>;
  hourly_rate?: Maybe<Scalars['Float']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  last_name?: Maybe<Scalars['String']['output']>;
  meta?: Maybe<EmployeeEmployee_Meta>;
  middle_name?: Maybe<Scalars['String']['output']>;
  phone?: Maybe<Scalars['String']['output']>;
  position?: Maybe<EmployeePositions>;
  position_id?: Maybe<Scalars['Int']['output']>;
  salary?: Maybe<Scalars['Float']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type EmployeeEmployee_Meta = {
  __typename?: 'EmployeeEmployee_meta';
  birth_date?: Maybe<Scalars['String']['output']>;
  employee_id?: Maybe<Scalars['Int']['output']>;
  general_info?: Maybe<Scalars['String']['output']>;
  join_date?: Maybe<Scalars['String']['output']>;
  leave_date?: Maybe<Scalars['String']['output']>;
  primary_language?: Maybe<Scalars['String']['output']>;
  secondary_language?: Maybe<Scalars['String']['output']>;
  tin?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
  yt_profile_id?: Maybe<Scalars['String']['output']>;
};

export type EmployeePositions = {
  __typename?: 'EmployeePositions';
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type EmployeeType = {
  __typename?: 'EmployeeType';
  avatar_id?: Maybe<Scalars['Int']['output']>;
  coefficient?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  first_name?: Maybe<Scalars['String']['output']>;
  hourly_rate?: Maybe<Scalars['Float']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  last_name?: Maybe<Scalars['String']['output']>;
  meta?: Maybe<EmployeeTypeEmployee_Meta>;
  middle_name?: Maybe<Scalars['String']['output']>;
  phone?: Maybe<Scalars['String']['output']>;
  position?: Maybe<EmployeeTypePositions>;
  position_id?: Maybe<Scalars['Int']['output']>;
  salary?: Maybe<Scalars['Float']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type EmployeeTypeEmployee_Meta = {
  __typename?: 'EmployeeTypeEmployee_meta';
  birth_date?: Maybe<Scalars['String']['output']>;
  employee_id?: Maybe<Scalars['Int']['output']>;
  general_info?: Maybe<Scalars['String']['output']>;
  join_date?: Maybe<Scalars['String']['output']>;
  leave_date?: Maybe<Scalars['String']['output']>;
  primary_language?: Maybe<Scalars['String']['output']>;
  secondary_language?: Maybe<Scalars['String']['output']>;
  tin?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
  yt_profile_id?: Maybe<Scalars['String']['output']>;
};

export type EmployeeTypePositions = {
  __typename?: 'EmployeeTypePositions';
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type EmployeesAggregate = {
  __typename?: 'EmployeesAggregate';
  avatar_id?: Maybe<EmployeesAvatar_IdAggregationQuery>;
  coefficient?: Maybe<EmployeesCoefficientAggregationQuery>;
  created_at?: Maybe<EmployeesCreated_AtAggregationQuery>;
  email?: Maybe<EmployeesEmailAggregationQuery>;
  first_name?: Maybe<EmployeesFirst_NameAggregationQuery>;
  hourly_rate?: Maybe<EmployeesHourly_RateAggregationQuery>;
  id?: Maybe<EmployeesIdAggregationQuery>;
  last_name?: Maybe<EmployeesLast_NameAggregationQuery>;
  middle_name?: Maybe<EmployeesMiddle_NameAggregationQuery>;
  phone?: Maybe<EmployeesPhoneAggregationQuery>;
  position_id?: Maybe<EmployeesPosition_IdAggregationQuery>;
  salary?: Maybe<EmployeesSalaryAggregationQuery>;
  updated_at?: Maybe<EmployeesUpdated_AtAggregationQuery>;
};


export type EmployeesAggregateAvatar_IdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type EmployeesAggregateCoefficientArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type EmployeesAggregateCreated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateEmailArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateFirst_NameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateHourly_RateArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type EmployeesAggregateIdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type EmployeesAggregateLast_NameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateMiddle_NameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregatePhoneArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregatePosition_IdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type EmployeesAggregateSalaryArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type EmployeesAggregateUpdated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type EmployeesAvatar_IdAggregationQuery = {
  __typename?: 'EmployeesAvatar_idAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesCoefficientAggregationQuery = {
  __typename?: 'EmployeesCoefficientAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type EmployeesCreated_AtAggregationQuery = {
  __typename?: 'EmployeesCreated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type EmployeesEmailAggregationQuery = {
  __typename?: 'EmployeesEmailAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesFirst_NameAggregationQuery = {
  __typename?: 'EmployeesFirst_nameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesHourly_RateAggregationQuery = {
  __typename?: 'EmployeesHourly_rateAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type EmployeesIdAggregationQuery = {
  __typename?: 'EmployeesIdAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesLast_NameAggregationQuery = {
  __typename?: 'EmployeesLast_nameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesMiddle_NameAggregationQuery = {
  __typename?: 'EmployeesMiddle_nameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesPhoneAggregationQuery = {
  __typename?: 'EmployeesPhoneAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesPosition_IdAggregationQuery = {
  __typename?: 'EmployeesPosition_idAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesSalaryAggregationQuery = {
  __typename?: 'EmployeesSalaryAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type EmployeesUpdated_AtAggregationQuery = {
  __typename?: 'EmployeesUpdated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type Expense = {
  __typename?: 'Expense';
  amount?: Maybe<Scalars['Float']['output']>;
  category?: Maybe<ExpenseExpense_Categories>;
  category_id?: Maybe<Scalars['Int']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  date?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type ExpenseCategory = {
  __typename?: 'ExpenseCategory';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type ExpenseCategoryType = {
  __typename?: 'ExpenseCategoryType';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type ExpenseExpense_Categories = {
  __typename?: 'ExpenseExpense_categories';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type ExpenseType = {
  __typename?: 'ExpenseType';
  amount?: Maybe<Scalars['Float']['output']>;
  category?: Maybe<ExpenseTypeExpense_Categories>;
  category_id?: Maybe<Scalars['Int']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  date?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type ExpenseTypeExpense_Categories = {
  __typename?: 'ExpenseTypeExpense_categories';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type Expense_CategoriesAggregate = {
  __typename?: 'Expense_categoriesAggregate';
  amount?: Maybe<Expense_CategoriesAmountAggregationQuery>;
  created_at?: Maybe<Expense_CategoriesCreated_AtAggregationQuery>;
  description?: Maybe<Expense_CategoriesDescriptionAggregationQuery>;
  id?: Maybe<Expense_CategoriesIdAggregationQuery>;
  name?: Maybe<Expense_CategoriesNameAggregationQuery>;
  updated_at?: Maybe<Expense_CategoriesUpdated_AtAggregationQuery>;
};


export type Expense_CategoriesAggregateAmountArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type Expense_CategoriesAggregateCreated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type Expense_CategoriesAggregateDescriptionArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type Expense_CategoriesAggregateIdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type Expense_CategoriesAggregateNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type Expense_CategoriesAggregateUpdated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type Expense_CategoriesAmountAggregationQuery = {
  __typename?: 'Expense_categoriesAmountAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type Expense_CategoriesCreated_AtAggregationQuery = {
  __typename?: 'Expense_categoriesCreated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type Expense_CategoriesDescriptionAggregationQuery = {
  __typename?: 'Expense_categoriesDescriptionAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type Expense_CategoriesIdAggregationQuery = {
  __typename?: 'Expense_categoriesIdAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type Expense_CategoriesNameAggregationQuery = {
  __typename?: 'Expense_categoriesNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type Expense_CategoriesUpdated_AtAggregationQuery = {
  __typename?: 'Expense_categoriesUpdated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type ExpensesAggregate = {
  __typename?: 'ExpensesAggregate';
  amount?: Maybe<ExpensesAmountAggregationQuery>;
  category_id?: Maybe<ExpensesCategory_IdAggregationQuery>;
  created_at?: Maybe<ExpensesCreated_AtAggregationQuery>;
  date?: Maybe<ExpensesDateAggregationQuery>;
  id?: Maybe<ExpensesIdAggregationQuery>;
  updated_at?: Maybe<ExpensesUpdated_AtAggregationQuery>;
};


export type ExpensesAggregateAmountArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type ExpensesAggregateCategory_IdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type ExpensesAggregateCreated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type ExpensesAggregateDateArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type ExpensesAggregateIdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type ExpensesAggregateUpdated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type ExpensesAmountAggregationQuery = {
  __typename?: 'ExpensesAmountAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type ExpensesCategory_IdAggregationQuery = {
  __typename?: 'ExpensesCategory_idAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type ExpensesCreated_AtAggregationQuery = {
  __typename?: 'ExpensesCreated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type ExpensesDateAggregationQuery = {
  __typename?: 'ExpensesDateAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type ExpensesIdAggregationQuery = {
  __typename?: 'ExpensesIdAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type ExpensesUpdated_AtAggregationQuery = {
  __typename?: 'ExpensesUpdated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type MetaInput = {
  birth_date?: InputMaybe<Scalars['String']['input']>;
  employee_id?: InputMaybe<Scalars['Int']['input']>;
  general_info?: InputMaybe<Scalars['String']['input']>;
  join_date?: InputMaybe<Scalars['String']['input']>;
  leave_date?: InputMaybe<Scalars['String']['input']>;
  primary_language?: InputMaybe<Scalars['String']['input']>;
  secondary_language?: InputMaybe<Scalars['String']['input']>;
  tin?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
  yt_profile_id?: InputMaybe<Scalars['String']['input']>;
};

export type Mutation = {
  __typename?: 'Mutation';
  authenticate?: Maybe<AuthPayload>;
  /** Create a record */
  createEmployee?: Maybe<EmployeeType>;
  /** Create a record */
  createExpense?: Maybe<ExpenseType>;
  /** Create a record */
  createExpenseCategory?: Maybe<ExpenseCategoryType>;
  /** Create a record */
  createPosition?: Maybe<PositionType>;
  /** Create a record */
  createTaskType?: Maybe<TaskTypeType>;
  /** Create a new user */
  createUser?: Maybe<UserType>;
  /** Delete a record */
  deleteEmployee?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deleteExpense?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deleteExpenseCategory?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deletePosition?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deleteTaskType?: Maybe<Scalars['String']['output']>;
  /** Delete a user */
  deleteUser?: Maybe<Scalars['Boolean']['output']>;
  /** Update a record */
  updateEmployee?: Maybe<EmployeeType>;
  /** Update a record */
  updateExpense?: Maybe<ExpenseType>;
  /** Update a record */
  updateExpenseCategory?: Maybe<ExpenseCategoryType>;
  /** Update a record */
  updatePosition?: Maybe<PositionType>;
  /** Update a record */
  updateTaskType?: Maybe<TaskTypeType>;
  /** Update a user */
  updateUser?: Maybe<UserType>;
};


export type MutationAuthenticateArgs = {
  email: Scalars['String']['input'];
  password: Scalars['String']['input'];
};


export type MutationCreateEmployeeArgs = {
  data?: InputMaybe<CreateEmployeeInput>;
};


export type MutationCreateExpenseArgs = {
  data?: InputMaybe<CreateExpenseInput>;
};


export type MutationCreateExpenseCategoryArgs = {
  data?: InputMaybe<CreateExpenseCategoryInput>;
};


export type MutationCreatePositionArgs = {
  data?: InputMaybe<CreatePositionInput>;
};


export type MutationCreateTaskTypeArgs = {
  data?: InputMaybe<CreateTaskTypeInput>;
};


export type MutationCreateUserArgs = {
  data: CreateUserInput;
};


export type MutationDeleteEmployeeArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteExpenseArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteExpenseCategoryArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeletePositionArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteTaskTypeArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteUserArgs = {
  id: Scalars['Int']['input'];
};


export type MutationUpdateEmployeeArgs = {
  data?: InputMaybe<UpdateEmployeeInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateExpenseArgs = {
  data?: InputMaybe<UpdateExpenseInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateExpenseCategoryArgs = {
  data?: InputMaybe<UpdateExpenseCategoryInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdatePositionArgs = {
  data?: InputMaybe<UpdatePositionInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateTaskTypeArgs = {
  data?: InputMaybe<UpdateTaskTypeInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateUserArgs = {
  data: UpdateUserInput;
  id: Scalars['Int']['input'];
};

export type Position = {
  __typename?: 'Position';
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type PositionInput = {
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type PositionType = {
  __typename?: 'PositionType';
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type PositionsAggregate = {
  __typename?: 'PositionsAggregate';
  created_at?: Maybe<PositionsCreated_AtAggregationQuery>;
  description?: Maybe<PositionsDescriptionAggregationQuery>;
  id?: Maybe<PositionsIdAggregationQuery>;
  name?: Maybe<PositionsNameAggregationQuery>;
  updated_at?: Maybe<PositionsUpdated_AtAggregationQuery>;
};


export type PositionsAggregateCreated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type PositionsAggregateDescriptionArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type PositionsAggregateIdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type PositionsAggregateNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type PositionsAggregateUpdated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type PositionsCreated_AtAggregationQuery = {
  __typename?: 'PositionsCreated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type PositionsDescriptionAggregationQuery = {
  __typename?: 'PositionsDescriptionAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type PositionsIdAggregationQuery = {
  __typename?: 'PositionsIdAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type PositionsNameAggregationQuery = {
  __typename?: 'PositionsNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type PositionsUpdated_AtAggregationQuery = {
  __typename?: 'PositionsUpdated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type Query = {
  __typename?: 'Query';
  /** Get by id */
  employee?: Maybe<Employee>;
  /** Get paginated */
  employees?: Maybe<Employees>;
  /** Get aggregated data */
  employeesAggregate?: Maybe<Array<Maybe<EmployeesAggregate>>>;
  /** Get by id */
  expense?: Maybe<Expense>;
  /** Get paginated */
  expenseCategories?: Maybe<ExpenseCategories>;
  /** Get aggregated data */
  expenseCategoriesAggregate?: Maybe<Array<Maybe<Expense_CategoriesAggregate>>>;
  /** Get by id */
  expenseCategory?: Maybe<ExpenseCategory>;
  /** Get paginated */
  expenses?: Maybe<Expenses>;
  /** Get aggregated data */
  expensesAggregate?: Maybe<Array<Maybe<ExpensesAggregate>>>;
  /** Get by id */
  position?: Maybe<Position>;
  /** Get paginated */
  positions?: Maybe<Positions>;
  /** Get aggregated data */
  positionsAggregate?: Maybe<Array<Maybe<PositionsAggregate>>>;
  /** Get by id */
  taskType?: Maybe<TaskType>;
  /** Get paginated */
  taskTypes?: Maybe<TaskTypes>;
  /** Get aggregated data */
  taskTypesAggregate?: Maybe<Array<Maybe<Task_TypesAggregate>>>;
  /** Get by id */
  user?: Maybe<User>;
  /** Get paginated */
  users?: Maybe<Users>;
};


export type QueryEmployeeArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryEmployeesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryEmployeesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryExpenseCategoriesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseCategoriesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseCategoryArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryExpensesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpensesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryPositionArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryPositionsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryPositionsAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryTaskTypeArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryTaskTypesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryTaskTypesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryUserArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryUsersArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type TaskType = {
  __typename?: 'TaskType';
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  icon?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type TaskTypeType = {
  __typename?: 'TaskTypeType';
  created_at?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  icon?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type Task_TypesAggregate = {
  __typename?: 'Task_typesAggregate';
  created_at?: Maybe<Task_TypesCreated_AtAggregationQuery>;
  description?: Maybe<Task_TypesDescriptionAggregationQuery>;
  icon?: Maybe<Task_TypesIconAggregationQuery>;
  id?: Maybe<Task_TypesIdAggregationQuery>;
  name?: Maybe<Task_TypesNameAggregationQuery>;
  updated_at?: Maybe<Task_TypesUpdated_AtAggregationQuery>;
};


export type Task_TypesAggregateCreated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type Task_TypesAggregateDescriptionArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type Task_TypesAggregateIconArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type Task_TypesAggregateIdArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type Task_TypesAggregateNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type Task_TypesAggregateUpdated_AtArgs = {
  gt?: InputMaybe<Scalars['String']['input']>;
  gte?: InputMaybe<Scalars['String']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  lt?: InputMaybe<Scalars['String']['input']>;
  lte?: InputMaybe<Scalars['String']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type Task_TypesCreated_AtAggregationQuery = {
  __typename?: 'Task_typesCreated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type Task_TypesDescriptionAggregationQuery = {
  __typename?: 'Task_typesDescriptionAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type Task_TypesIconAggregationQuery = {
  __typename?: 'Task_typesIconAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type Task_TypesIdAggregationQuery = {
  __typename?: 'Task_typesIdAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type Task_TypesNameAggregationQuery = {
  __typename?: 'Task_typesNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type Task_TypesUpdated_AtAggregationQuery = {
  __typename?: 'Task_typesUpdated_atAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['String']['output']>;
  min?: Maybe<Scalars['String']['output']>;
};

export type UpdateEmployeeInput = {
  avatar_id?: InputMaybe<Scalars['Int']['input']>;
  coefficient?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  first_name?: InputMaybe<Scalars['String']['input']>;
  hourly_rate?: InputMaybe<Scalars['Float']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  last_name?: InputMaybe<Scalars['String']['input']>;
  meta?: InputMaybe<MetaInput>;
  middle_name?: InputMaybe<Scalars['String']['input']>;
  phone?: InputMaybe<Scalars['String']['input']>;
  position?: InputMaybe<PositionInput>;
  position_id?: InputMaybe<Scalars['Int']['input']>;
  salary?: InputMaybe<Scalars['Float']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateExpenseCategoryInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateExpenseInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  category?: InputMaybe<CategoryInput>;
  category_id?: InputMaybe<Scalars['Int']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  date?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type UpdatePositionInput = {
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateTaskTypeInput = {
  created_at?: InputMaybe<Scalars['String']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  icon?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateUserInput = {
  avatar?: InputMaybe<AvatarInput>;
  avatar_id?: InputMaybe<Scalars['Int']['input']>;
  created_at?: InputMaybe<Scalars['String']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  employee_id?: InputMaybe<Scalars['Int']['input']>;
  first_name?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  last_action?: InputMaybe<Scalars['String']['input']>;
  last_ip?: InputMaybe<Scalars['String']['input']>;
  last_login?: InputMaybe<Scalars['String']['input']>;
  last_name?: InputMaybe<Scalars['String']['input']>;
  middle_name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['String']['input']>;
};

export type User = {
  __typename?: 'User';
  avatar?: Maybe<UserUploads>;
  avatar_id?: Maybe<Scalars['Int']['output']>;
  created_at?: Maybe<Scalars['String']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  employee_id?: Maybe<Scalars['Int']['output']>;
  first_name?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  last_action?: Maybe<Scalars['String']['output']>;
  last_ip?: Maybe<Scalars['String']['output']>;
  last_login?: Maybe<Scalars['String']['output']>;
  last_name?: Maybe<Scalars['String']['output']>;
  middle_name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
};

export type UserType = {
  __typename?: 'UserType';
  avatar_id?: Maybe<Scalars['Int']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  first_name?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  last_name?: Maybe<Scalars['String']['output']>;
};

export type UserUploads = {
  __typename?: 'UserUploads';
  created_at?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  mimetype?: Maybe<Scalars['String']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  path?: Maybe<Scalars['String']['output']>;
  size?: Maybe<Scalars['Float']['output']>;
  updated_at?: Maybe<Scalars['String']['output']>;
  uploader_id?: Maybe<Scalars['Int']['output']>;
};

export type Employees = {
  __typename?: 'employees';
  data?: Maybe<Array<Maybe<Employee>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type ExpenseCategories = {
  __typename?: 'expenseCategories';
  data?: Maybe<Array<Maybe<ExpenseCategory>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type Expenses = {
  __typename?: 'expenses';
  data?: Maybe<Array<Maybe<Expense>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type Positions = {
  __typename?: 'positions';
  data?: Maybe<Array<Maybe<Position>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type TaskTypes = {
  __typename?: 'taskTypes';
  data?: Maybe<Array<Maybe<TaskType>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type Users = {
  __typename?: 'users';
  data?: Maybe<Array<Maybe<User>>>;
  total?: Maybe<Scalars['Int']['output']>;
};
