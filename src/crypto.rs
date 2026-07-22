use zeroize::{Zeroize, ZeroizeOnDrop};

#[derive(Zeroize, ZeroizeOnDrop)]
pub struct SecretKey256(pub [u8; 32]);

impl SecretKey256 {
    pub fn from_slice(slice: &[u8]) -> Self {
        let mut key = [0u8; 32];
        let len = slice.len().min(32);
        key[..len].copy_from_slice(&slice[..len]);
        Self(key)
    }

    pub fn as_bytes(&self) -> &[u8; 32] {
        &self.0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_secret_key_zeroization() {
        let ptr: *const u8;
        
        {
            let key = SecretKey256::from_slice(&[0xAA; 32]);
            ptr = key.as_bytes().as_ptr();
            
            unsafe {
                assert_eq!(*ptr, 0xAA);
            }
        } 

        unsafe {
            assert_eq!(*ptr, 0x00);
        }
    }
}