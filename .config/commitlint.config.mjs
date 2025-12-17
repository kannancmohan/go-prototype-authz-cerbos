export default {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // Type definitions - using conventional commits
    'type-enum': [
      2,
      'always',
      [
        'build',
        'chore',
        'ci',
        'docs',
        'feat',
        'fix',  
        'perf',
        'refactor',
        'revert',
        'style',
        'test',
        'wip'
      ]
    ],
    
    // Type must be lowercase
    'type-case': [2, 'always', 'lower-case'],
    
    // Type cannot be empty
    'type-empty': [2, 'never'],
    
    // Scope must be lowercase
    'scope-case': [2, 'always', 'lower-case'],
    
    // Scope can be empty but if provided must follow format
    'scope-empty': [0],
    
    // Subject must not be empty
    'subject-empty': [2, 'never'],
    
    // Subject must not end with period
    'subject-full-stop': [2, 'never', '.'],
    
    // Subject must start with lowercase
    'subject-case': [
      2,
      'never',
      ['sentence-case', 'start-case', 'pascal-case', 'upper-case']
    ],
    
    // Header length limits
    'header-max-length': [2, 'always', 100],
    'header-min-length': [2, 'always', 10],
    
    // Body lines should not be longer than 100 characters
    'body-max-line-length': [2, 'always', 100],
    
    // Footer lines should not be longer than 100 characters
    'footer-max-line-length': [2, 'always', 100],
    
    // Body must have leading blank line
    'body-leading-blank': [2, 'always'],
    
    // Footer must have leading blank line
    'footer-leading-blank': [2, 'always'],
    
    // References issues/PRs must be formatted properly
    'references-empty': [0], // Can be empty
    
    // Signed-off-by is optional
    'signed-off-by': [0]
  },
  
  // Help URL for users when commit fails
  helpUrl: 'https://github.com/conventional-changelog/commitlint/#what-is-commitlint',
  
  // Default ignores for certain commit messages
  ignores: [
    (commit) => commit === '', // Ignore empty commits
    (commit) => commit.startsWith('Merge branch'), // Ignore merge commits
    (commit) => commit.startsWith('Merge pull request'), // Ignore PR merges
    (commit) => commit.startsWith('Revert'), // Ignore revert commits
  ]
};