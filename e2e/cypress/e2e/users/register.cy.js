/// <reference types="cypress" />

describe('user auth and registration flow', () => {
    beforeEach(() => {
        cy.visit('http://localhost:3200/login')
        cy.get('[type=email]').type('test@gmail.com')
        cy.get('[type=password]').type('TestPass123!')
        cy.get('[type=submit]').click()
        cy.visit('http://localhost:3200/users')
    })

    afterEach(() => {
        cy.visit('http://localhost:3200/logout')
    })

    it('creates a user and displays changes in users table', () => {
        cy.get('[href="/users/new"]').click()
        cy.get('[name=FirstName]').type('Test')
        cy.get('[name=LastName]').type('User')
        cy.get('[name=MiddleName]').type('Mid')
        cy.get('[name=Email]').type('test1@gmail.com')
        cy.get('[name=Password]').type('TestPass123!')
        cy.get('[name=UILanguage]').select(2)
        cy.get('[x-ref=trigger]').click()
        cy.get('ul[x-ref=list]').find('li').first().click()
        cy.get('[id=save-btn]').click()
        cy.visit('http://localhost:3200/users')
        cy.get('tbody tr').should('have.length', 2)
        cy.visit('http://localhost:3200/logout')
        cy.visit('http://localhost:3200/login')
        cy.get('[type=email]').type('test1@gmail.com')
        cy.get('[type=password]').type('TestPass123!')
        cy.get('[type=submit]').click()
        cy.visit('http://localhost:3200/users')

        cy.url().should('include', '/users')
        cy.get('tbody tr').should('have.length', 2)
    })
})
